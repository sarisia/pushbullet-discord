package main

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	"nhooyr.io/websocket"
)

const PushbulletStreamEndpoint = "wss://stream.pushbullet.com/websocket"

// PayloadHeader represents JSON struct sent from
// Realtime Event Stream
// https://docs.pushbullet.com/#realtime-event-stream
type PayloadHeader struct {
	Type string `json:"type"`
}

type NopPayload struct {
	PayloadHeader
}

type TicklePayload struct {
	PayloadHeader
	Subtype string `json:"subtype"`
}

type PushPayload struct {
	PayloadHeader
	Push PushMessage
}

type PushMessage struct {
	Type            string `json:"type"`
	ApplicationName string `json:"application_name"`
	Title           string `json:"title"`
	Body            string `json:"body"`
	PackageName     string `json:"package_name"`
}

type payloadHandler interface {
	typ() string
	handle(context.Context, interface{})
}

type PushPayloadHandler func(context.Context, *PushPayload)

func (ph PushPayloadHandler) typ() string {
	return "push"
}
func (ph PushPayloadHandler) handle(ctx context.Context, p interface{}) {
	if packet, ok := p.(*PushPayload); ok {
		ph(ctx, packet)
	}
}

func handlerForFunc(i interface{}) payloadHandler {
	switch f := i.(type) {
	case func(context.Context, *PushPayload):
		return PushPayloadHandler(f)
	}

	return nil
}

type PushbulletClient struct {
	// args
	token string

	// internal field
	cancel    func()
	handlerMu sync.RWMutex
	wg        sync.WaitGroup
	handlers  map[string][]payloadHandler
}

func NewPushbulletClient(token string) *PushbulletClient {
	cli := PushbulletClient{}
	cli.token = token
	cli.handlers = make(map[string][]payloadHandler)
	return &cli
}

func (c *PushbulletClient) AddHandler(i interface{}) {
	// TODO: return "remove handle" function
	h := handlerForFunc(i)
	if h == nil {
		log.Printf("invalid handler")
		return
	}

	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.handlers[h.typ()] = append(c.handlers[h.typ()], h)
}

func (c *PushbulletClient) open(syncExec bool) {
	if c.cancel != nil {
		log.Printf("already opened")
		return
	}

	url := PushbulletStreamEndpoint + "/" + c.token
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		log.Printf("failed to open websocket to Pushbullet: %v", err)
		return
	}

	c.wg.Add(1)
	go func() {
		c.recvLoop(ctx, conn)
		c.wg.Done()
	}()

	if syncExec {
		c.wg.Wait()
	}
}

func (c *PushbulletClient) Open() {
	c.open(false)
}

func (c *PushbulletClient) OpenSync() {
	c.open(true)
}

func (c *PushbulletClient) Close() {
	if c.cancel == nil {
		return
	}

	c.cancel()
	c.cancel = nil
	c.wg.Wait()
}

func (c *PushbulletClient) Wait() {
	c.wg.Wait()
}

func (c *PushbulletClient) recvLoop(ctx context.Context, conn *websocket.Conn) {
	for {
		// do not use Reader to avoid blocking websocket
		typ, b, err := conn.Read(ctx)
		if err != nil {
			log.Printf("failed to get reader: %v", err)
			break
		}

		go c.handlePacket(ctx, typ, b)
	}
}

func (c *PushbulletClient) handlePacket(ctx context.Context, typ websocket.MessageType, b []byte) {
	if typ != websocket.MessageText {
		return
	}

	// we do not use wsjson to parse body since we need to parse response twice
	// in order to determine which fields they contains

	// parse header
	header := PayloadHeader{}
	if err := json.Unmarshal(b, &header); err != nil {
		log.Printf("failed to unmarshal PayloadHeader: %v", err)
		return
	}

	// TODO: do generic unmarshaller & handler
	switch header.Type {
	// case "nop":
	// 	c.handleNop(ctx, b)
	// case "tickle":
	// 	c.handleTickle(ctx, b)
	case "push":
		c.handlePush(ctx, b)
	}
}

func (c *PushbulletClient) handlePush(ctx context.Context, b []byte) {
	push := PushPayload{}
	if err := json.Unmarshal(b, &push); err != nil {
		log.Printf("failed to unmarshal PushPayload: %v", err)
		return
	}

	log.Printf("%+v\n", push)

	c.handlerMu.RLock()
	defer c.handlerMu.RUnlock()
	if handlers, ok := c.handlers["push"]; ok {
		for _, h := range handlers {
			go h.handle(ctx, &push)
		}
	}
}
