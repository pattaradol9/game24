package ws

import "crypto/rand"

func randRead(b []byte) error {
	_, err := rand.Read(b)
	return err
}
