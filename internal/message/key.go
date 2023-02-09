package message

import (
	"encoding/base64"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

type messageKey string

const (
	messageKeyEventPrefix = "event"
	messageKeyLocalPrefix = "local"
)

func messageKeyRow(row *gtk.ListBoxRow) messageKey {
	if row == nil {
		return ""
	}

	name := row.Name()
	if !strings.Contains(name, ":") {
		log.Panicf("row name %q not a messageKey", name)
	}

	return messageKey(name)
}

// messageKeyID returns the messageKey for a message ID.
func messageKeyID(id discord.MessageID) messageKey {
	return messageKey(messageKeyEventPrefix + ":" + string(id.String()))
}

var (
	messageKeyLocalInc uint64
	messageNonceRandom = rand.New(rand.NewSource(time.Now().UnixNano()))
)

// messageKeyLocal creates a new local messageKey that will never collide with
// server events.
func messageKeyLocal() messageKey {
	inc := atomic.AddUint64(&messageKeyLocalInc, 1)
	num := strconv.FormatUint(inc, 32)

	var rand [8]byte
	messageNonceRandom.Read(rand[:])
	prefix := base64.RawStdEncoding.EncodeToString(rand[:])

	return messageKey(fmt.Sprintf(
		"%s:%s-%s", messageKeyLocalPrefix, prefix, num,
	))
}

// messageKeyNonce creates a new messageKey from the given nonce.
func messageKeyNonce(nonce string) messageKey {
	return messageKey(messageKeyLocalPrefix + ":" + nonce)
}

func (k messageKey) parts() (typ, val string) {
	parts := strings.SplitN(string(k), ":", 2)
	if len(parts) != 2 {
		log.Panicf("invalid messageKey %q", parts)
	}
	return parts[0], parts[1]
}

// ID takes the message ID from the message key. If the key doesn't hold an
// event ID, then it panics.
func (k messageKey) ID() discord.MessageID {
	t, v := k.parts()
	if t != messageKeyEventPrefix {
		panic("EventID called on non-event message key")
	}

	u, err := discord.ParseSnowflake(v)
	if err != nil {
		panic("invalid snowflake stored: " + err.Error())
	}

	return discord.MessageID(u)
}

// Nonce returns a nonce from the message key. The nonce has a static size. If
// the key doesn't hold a local value, then it panics.
func (k messageKey) Nonce() string {
	t, v := k.parts()
	if t != messageKeyLocalPrefix {
		panic("Nonce called on non-local message key")
	}
	return v
}

func (k messageKey) IsEvent() bool {
	typ, _ := k.parts()
	return typ == messageKeyEventPrefix
}

func (k messageKey) IsLocal() bool {
	typ, _ := k.parts()
	return typ == messageKeyLocalPrefix
}
