package message

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis"
	"github.com/sirupsen/logrus"
)

const (
	globalChannelPrefix               = "tw-global-"
	workerChannelPrefix               = "tw-worker-"
	ipKeyPrefix                       = "tw-ip-"
	DefaultMsgExpireTime              = 24 * time.Hour
	DefaultRetryIntervalForGetWaiting = 2 * time.Second

	EnvRedisAddr     = "REDIS_ADDR"
	EnvRedisDB       = "REDIS_DB"
	EnvRedisPassword = "REDIS_PASS"
)

type Message struct {
	Client  *redis.Client
	NodeUID string // the UID is used to identify the node and to receive messages
}

// Send sends a message to the receiver with the given UID.
// Please note that the other node must be listening to the same key to receive the message.
func (m *Message) Send(receiverUID, key string, value interface{}) error {
	redisKey := workerChannelPrefix + receiverUID + key
	return m.publish(redisKey, value)
}

// OnReceive sets a handler to receive messages from the given key.
// The handler will be called when a message is received from the given key.
func (m *Message) OnReceive(key string, handler func(value interface{})) {
	redisKey := workerChannelPrefix + m.NodeUID + key
	m.setOnReceive(redisKey, handler)
}

// Receive receives a message from the given key.
// It waits until a message is received from the given key.
func (m *Message) Receive(key string) (interface{}, error) {
	redisKey := workerChannelPrefix + m.NodeUID + key
	return m.receive(redisKey)
}

// SendGlobal sends a message to all nodes with the given key.
// Please note that the other nodes must be listening to the same key to receive the message.
func (m *Message) SendGlobal(key string, value interface{}) error {
	redisKey := globalChannelPrefix + key
	return m.publish(redisKey, value)
}

// OnReceiveGlobal sets a handler to receive messages from the given key.
// The handler will be called when a message is received from the given key.
func (m *Message) OnReceiveGlobal(key string, handler func(value interface{})) {
	redisKey := globalChannelPrefix + key
	m.setOnReceive(redisKey, handler)
}

// ReceiveGlobal receives a message from the given key.
// It waits until a message is received from the given key.
func (m *Message) ReceiveGlobal(key string) (interface{}, error) {
	redisKey := globalChannelPrefix + key
	return m.receive(redisKey)
}

// SetIP sets the IP address of the node that is calling it.
func (m *Message) SetIP(ipAddr string) error {
	redisKey := ipKeyPrefix + m.NodeUID
	return m.Set(redisKey, ipAddr)
}

// GetIP gets the IP address of a node with the given UID.
func (m *Message) GetIP(nodeUID string) (string, error) {
	return m.GetIPWaiting(nil, nodeUID)
}

// GetIPWaiting gets the IP address of a node with the given UID.
// It waits until either the IP address is set by the node or the context is done.
func (m *Message) GetIPWaiting(ctx context.Context, nodeUID string) (string, error) {
	var (
		redisKey = ipKeyPrefix + nodeUID
		value    interface{}
		err      error
	)
	if ctx == nil {
		value, err = m.Get(redisKey)
	} else {
		value, err = m.GetWaiting(ctx, redisKey)
	}
	if err != nil {
		if err == ErrMsgNotFound {
			return "", err
		}
		return "", ErrGetIP.Wrap(err)
	}

	ipAddr, ok := value.(string)
	if !ok {
		return "", ErrTypeCasting.Wrap(
			fmt.Errorf(
				"Expecting to get a string type for IP address, got `%T`",
				value,
			),
		)
	}

	return ipAddr, nil
}

// Set sets the value of the given key in Redis.
func (m *Message) Set(redisKey string, value interface{}) error {
	err := m.Client.Set(redisKey, value, DefaultMsgExpireTime).Err()
	if err != nil {
		return ErrSetIP.Wrap(err)
	}
	return nil
}

// Get gets the value of the given key from Redis.
func (m *Message) Get(redisKey string) (interface{}, error) {
	value, err := m.Client.Get(redisKey).Result()
	if err != nil {
		if err == redis.Nil {
			return "", ErrMsgNotFound
		}
		return "", err
	}
	return value, nil
}

// GetWaiting gets the value of the given key from Redis.
// It waits until the value is set by someone or the context is done.
func (m *Message) GetWaiting(ctx context.Context, redisKey string) (interface{}, error) {
	ticker := time.NewTicker(DefaultRetryIntervalForGetWaiting)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ErrCtxDone
		default:
		}

		value, err := m.Get(redisKey)
		if err == nil {
			return value, nil
		}

		if err != ErrMsgNotFound {
			return nil, err
		}

		select {
		case <-ctx.Done():
			return nil, ErrCtxDone
		case <-ticker.C:
		}
	}
}

func (m *Message) publish(redisKey string, value interface{}) error {
	pub := m.Client.Publish(redisKey, value)
	if pub.Err() != nil {
		return ErrPublish.Wrap(pub.Err())
	}

	receivers, err := pub.Result()
	if err != nil {
		return ErrGetPubResult.Wrap(err)
	}
	if receivers == 0 {
		return ErrNoSubscribers
	}

	return nil
}

func (m *Message) setOnReceive(redisKey string, handler func(value interface{})) {
	go func() {
		pubsub := m.Client.Subscribe(redisKey)
		defer pubsub.Close()

		msg, err := pubsub.ReceiveMessage()
		if err != nil {
			logrus.Errorf("setOnReceive: receiving message: %v", err)
			return
		}

		handler(msg.Payload)

		if err := pubsub.Unsubscribe(redisKey); err != nil {
			logrus.Errorf("setOnReceive: unsubscribing from channel: %v", err)
		}
	}()
}

func (m *Message) receive(redisKey string) (interface{}, error) {
	pubsub := m.Client.Subscribe(redisKey)
	defer pubsub.Close()

	msg, err := pubsub.ReceiveMessage()
	if err != nil {
		return nil, err
	}

	if err := pubsub.Unsubscribe(redisKey); err != nil {
		return nil, err
	}
	return msg.Payload, nil
}
