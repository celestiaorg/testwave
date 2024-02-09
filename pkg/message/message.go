package message

import (
	"time"

	"github.com/go-redis/redis"
	"github.com/sirupsen/logrus"
)

const (
	globalChannelPrefix  = "tw-global-"
	workerChannelPrefix  = "tw-worker-"
	ipKeyPrefix          = "tw-ip-"
	DefaultMsgExpireTime = 24 * time.Hour

	EnvRedisAddr     = "REDIS_ADDR"
	EnvRedisDB       = "REDIS_DB"
	EnvRedisPassword = "REDIS_PASS"
)

type Message struct {
	Client  *redis.Client
	NodeUID string // the UID is used to identify the node and to receive messages
}

func (m *Message) Send(receiverUID, key string, value interface{}) error {
	redisKey := workerChannelPrefix + receiverUID + key
	return m.publish(redisKey, value)
}

func (m *Message) OnReceive(key string, handler func(value interface{})) {
	redisKey := workerChannelPrefix + m.NodeUID + key
	m.setOnReceive(redisKey, handler)
}

func (m *Message) Receive(key string) (interface{}, error) {
	redisKey := workerChannelPrefix + m.NodeUID + key
	return m.receive(redisKey)
}

func (m *Message) SendGlobal(key string, value interface{}) error {
	redisKey := globalChannelPrefix + key
	return m.publish(redisKey, value)
}

func (m *Message) OnReceiveGlobal(key string, handler func(value interface{})) {
	redisKey := globalChannelPrefix + key
	m.setOnReceive(redisKey, handler)
}

func (m *Message) ReceiveGlobal(key string) (interface{}, error) {
	redisKey := globalChannelPrefix + key
	return m.receive(redisKey)
}

func (m *Message) SetIP(ipAddr string) error {
	redisKey := ipKeyPrefix + m.NodeUID
	err := m.Client.Set(redisKey, ipAddr, DefaultMsgExpireTime).Err()
	if err != nil {
		return ErrSetIP.Wrap(err)
	}
	return nil
}

func (m *Message) GetIP(nodeUID string) (string, error) {
	redisKey := ipKeyPrefix + nodeUID
	ipAddr, err := m.Client.Get(redisKey).Result()
	if err != nil {
		if err == redis.Nil {
			return "", ErrMsgNotFound
		}
		return "", ErrGetIP.Wrap(err)
	}
	return ipAddr, nil
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
