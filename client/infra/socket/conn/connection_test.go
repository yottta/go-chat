package conn

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

func TestReadNetworkMessage(t *testing.T) {
	errorTests := []struct {
		given       string
		expectedErr string
		readerFunc  func() io.Reader
	}{
		{
			given:       "an empty reader",
			expectedErr: "EOF",
			readerFunc: func() io.Reader {
				return bytes.NewReader([]byte{})
			},
		},
		{
			given:       "a reader with a shorter content than expected for the size",
			expectedErr: "unexpected EOF",
			readerFunc: func() io.Reader {
				return bytes.NewReader([]byte{1, 2, 3})
			},
		},
		{
			given:       "a reader with not a valid string formatted uint16",
			expectedErr: "invalid content message as the size of the message is unparseable: strconv.Atoi: parsing \"defin\": invalid syntax",
			readerFunc: func() io.Reader {
				return bytes.NewReader([]byte("definitely not a number"))
			},
		},
		{
			given:       "a reader containing a size but the size of the actual payload is less than indicated",
			expectedErr: "unexpected EOF",
			readerFunc: func() io.Reader {
				return bytes.NewReader([]byte("00009payload"))
			},
		},
		{
			given:       "a reader containing a size but the actual payload is in unparseable",
			expectedErr: "unexpected EOF",
			readerFunc: func() io.Reader {
				return bytes.NewReader([]byte("00021payloadpayloadpayloaddddddddd"))
			},
		},
	}

	for i := range errorTests {
		t.Run(fmt.Sprintf(`Given %s, When ReadNetworkMessage called, Then '%s' error expected`,
			errorTests[i].given,
			errorTests[i].expectedErr), func(t *testing.T) {
			_, err := ReadNetworkMessage(errorTests[i].readerFunc())
			if err == nil {
				t.Errorf("expected an error but received nothing")
			}
			if !strings.Contains(err.Error(), errorTests[i].expectedErr) {
				t.Errorf("expected '%s' to contain '%s'", err.Error(), errorTests[i].expectedErr)
			}
		})
	}

	t.Run(`Given an expected reader, When ReadNetworkMessage is called, Then the network message is returned correctly`, func(t *testing.T) {
		// Given
		msg := NetworkMsg{
			UserId:  "user_id",
			ChatId:  "chat_id",
			Message: "here is your message",
			At:      time.Now().UTC(),
		}
		var b bytes.Buffer
		if err := gob.NewEncoder(&b).Encode(msg); err != nil {
			t.Errorf("failed to encode message to send it over network: %s", err)
			t.FailNow()
			return
		}

		// When
		contentBytes := b.Bytes()
		size := fmt.Sprintf("%05d", len(contentBytes))
		fullContent := append([]byte(size), contentBytes...)
		decodedMsg, err := ReadNetworkMessage(bytes.NewReader(fullContent))
		if err != nil {
			t.Errorf("expected no error but received: %s", err)
			t.FailNow()
		}

		// Then
		if *decodedMsg != msg {
			t.Errorf("expected the decoded message to be equal with the one before encoding. expected: %s, actual: %s", msg, *decodedMsg)
			t.FailNow()
		}
	})
}
