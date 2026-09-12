// Writes a local synthetic-test configuration without printing secrets.
package main

import (
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	webpush "github.com/SherClockHolmes/webpush-go"
	"os"
)

func main() {
	out := flag.String("out", ".env.demo.local", "new ignored local environment file")
	flag.Parse()
	private, public, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		panic(err)
	}
	secret := make([]byte, 32)
	if _, err = rand.Read(secret); err != nil {
		panic(err)
	}
	code := base64.RawURLEncoding.EncodeToString(secret)
	body := fmt.Sprintf("APP_MODE=demo\nNOTIFICATION_MODE=TEST_ALLOWLIST\nSTAFF_AUTH_MODE=demo\nSESSION_SECRET=%s\nDATABASE_URL=postgres://balsuraksha:balsuraksha@localhost:5454/balsuraksha?sslmode=disable\nAPI_PORT=8080\nAI_ENABLED=false\nVAPID_PRIVATE_KEY=%s\nVAPID_PUBLIC_KEY=%s\nVAPID_SUBJECT=mailto:synthetic-test@example.test\nTEST_ENROLLMENT_CODE=%s\n", code, private, public, code)
	f, err := os.OpenFile(*out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if _, err = f.WriteString(body); err != nil {
		panic(err)
	}
	fmt.Println("Local test configuration created. Keep it private; share only the invitation with consenting testers.")
}
