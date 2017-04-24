package pubsub

import (
	"errors"
	"fmt"

	"github.com/TykTechnologies/tyk-cluster-framework/client"
	"github.com/TykTechnologies/tyk-cluster-framework/encoding"
	"github.com/TykTechnologies/tyk-cluster-framework/payloads"
	"github.com/TykTechnologies/tyk-cluster-framework/server"
)

// PSServer wraps a server instance
type PSServer struct {
	server server.Server
}

// PSClient wraps a client instance and keeps a register of message handlers to ensure
// that if a master change occurs, and a new connection pool is created, the handlers
// are transitioned ot the new connection
type PSClient struct {
	client      client.Client
	isConnected bool
	handlerPool map[string]client.PayloadHandler
}

// NewPSClient will return a ready-made PSClient handle
func NewPSClient() *PSClient {
	pc := &PSClient{}
	pc.handlerPool = make(map[string]client.PayloadHandler)
	return pc
}

// NewPSServer will return a new PSServer isntacne that is ready for listening
func NewPSServer(onPort string) (*PSServer, error) {
	cs := fmt.Sprintf("mangos://127.0.0.1:%v", onPort)
	s, err := server.NewServer(cs, encoding.JSON)
	if err != nil {
		return nil, err
	}

	pss := PSServer{
		server: s,
	}

	return &pss, nil
}

// Start will terminate any existing connections, and start a new client,
// if there are already any message handlers that have been registered, it will re-create them.
func (c *PSClient) Start(cs string) error {
	if c.client != nil && c.isConnected {
		err := c.Stop()
		if err != nil {
			return err
		}
	}

	// Create a new client from scratch because we might be reconnecting
	mc, err := client.NewClient(cs, encoding.JSON)
	if err != nil {
		return err
	}

	c.client = mc
	err = c.client.Connect()
	if err != nil {
		return err
	}

	c.isConnected = true

	// Initialise the subscriptions in case we are reconnecting
	for t, h := range c.handlerPool {
		_, err := c.client.Subscribe(t, h)
		if err == nil {
			return err
		}
	}

	return nil
}

// Stop will termiante a client conneciton
func (c *PSClient) Stop() error {
	err := c.client.Stop()
	if err != nil {
		return err
	}
	c.isConnected = false
	return nil
}

// Subscribe wraps a the client subscribe interface, and keeps track of handlers and topics
func (c *PSClient) Subscribe(topic string, handler client.PayloadHandler) error {
	_, f := c.handlerPool[topic]
	if f {
		// Error, already set
		return errors.New("Topic already has a handler!")
	}

	// Add it to our pool for later and do the sub
	c.handlerPool[topic] = handler
	if c.isConnected {
		// We are connected, so we should now actually do the sub
		_, err := c.client.Subscribe(topic, handler)
		if err != nil {
			return err
		}
	}
	return nil
}

// Publish will publish a message to a topic
func (c *PSClient) Publish(filter string, payload payloads.Payload) error {
	if !c.isConnected || c.client == nil {
		// TODO: should we queue them?
		return errors.New("Client is not connected!")
	}

	return c.client.Publish(filter, payload)
}

// Start will start a server
func (s *PSServer) Start() error {
	return s.server.Listen()
}

// Stop will stop the server
func (s *PSServer) Stop() error {
	return s.server.Stop()
}

// Publish wraps the publishing capability of the underlying server
func (s *PSServer) Publish(topic string, payload payloads.Payload) error {
	return s.server.Publish(topic, payload)
}
