// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package note

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"testing"
	"testing/iotest"
)

func TestNewVerifier(t *testing.T) {
	vkey := "PeterNeumann+c74f20a3+ARpc2QcUPDhMQegwxbzhKqiBfsVkmqq/LDE4izWy10TW"
	_, err := NewVerifier(vkey)
	if err != nil {
		t.Fatal(err)
	}

	// Check various manglings are not accepted.
	badKey := func(k string) {
		_, err := NewVerifier(k)
		if err == nil {
			t.Errorf("NewVerifier(%q) succeeded, should have failed", k)
		}
	}

	b := []byte(vkey)
	for i := 0; i <= len(b); i++ {
		for j := i + 1; j <= len(b); j++ {
			if i != 0 || j != len(b) {
				badKey(string(b[i:j]))
			}
		}
	}
	for i := 0; i < len(b); i++ {
		b[i]++
		badKey(string(b))
		b[i]--
	}

	badKey("PeterNeumann+cc469956+ARpc2QcUPDhMQegwxbzhKqiBfsVkmqq/LDE4izWy10TWBADKEY==") // wrong length key, with adjusted key hash
	badKey("PeterNeumann+173116ae+ZRpc2QcUPDhMQegwxbzhKqiBfsVkmqq/LDE4izWy10TW")         // unknown algorithm, with adjusted key hash
}

func TestNewSigner(t *testing.T) {
	skey := "PRIVATE+KEY+PeterNeumann+c74f20a3+AYEKFALVFGyNhPJEMzD1QIDr+Y7hfZx09iUvxdXHKDFz"
	_, err := NewSigner(skey)
	if err != nil {
		t.Fatal(err)
	}

	// Check various manglings are not accepted.
	b := []byte(skey)
	for i := 0; i <= len(b); i++ {
		for j := i + 1; j <= len(b); j++ {
			if i == 0 && j == len(b) {
				continue
			}
			_, err := NewSigner(string(b[i:j]))
			if err == nil {
				t.Errorf("NewSigner(%q) succeeded, should have failed", b[i:j])
			}
		}
	}
	for i := 0; i < len(b); i++ {
		b[i]++
		_, err := NewSigner(string(b))
		if err == nil {
			t.Errorf("NewSigner(%q) succeeded, should have failed", b)
		}
		b[i]--
	}
}

func TestGenerateKey(t *testing.T) {
	// Generate key pair, make sure it is all self-consistent.
	const Name = "EnochRoot"
	skey, vkey, err := GenerateKey(rand.Reader, Name)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	signer, err := NewSigner(skey)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	verifier, err := NewVerifier(vkey)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}

	if name := signer.Name(); name != Name {
		t.Errorf("signer.Name() = %q, want %q", name, Name)
	}
	if name := verifier.Name(); name != Name {
		t.Errorf("verifier.Name() = %q, want %q", name, Name)
	}
	shash := signer.KeyHash()
	vhash := verifier.KeyHash()
	if shash != vhash {
		t.Errorf("signer.KeyHash() = %#08x != verifier.KeyHash() = %#08x", shash, vhash)
	}

	msg := []byte("hi")
	sig, err := signer.Sign(msg)
	if err != nil {
		t.Fatalf("signer.Sign: %v", err)
	}
	if !verifier.Verify(msg, sig) {
		t.Fatalf("verifier.Verify failed on signature returned by signer.Sign")
	}
	sig[0]++
	if verifier.Verify(msg, sig) {
		t.Fatalf("verifier.Verify succceeded on corrupt signature")
	}

	// Check that GenerateKey returns error from rand reader.
	_, _, err = GenerateKey(iotest.TimeoutReader(iotest.OneByteReader(rand.Reader)), Name)
	if err == nil {
		t.Fatalf("GenerateKey succeeded with error-returning rand reader")
	}
}

func TestSign(t *testing.T) {
	skey := "PRIVATE+KEY+PeterNeumann+c74f20a3+AYEKFALVFGyNhPJEMzD1QIDr+Y7hfZx09iUvxdXHKDFz"
	text := "If you think cryptography is the answer to your problem,\n" +
		"then you don't know what your problem is.\n"

	signer, err := NewSigner(skey)
	if err != nil {
		t.Fatal(err)
	}

	msg, err := Sign(&Note{Text: text}, signer)
	if err != nil {
		t.Fatal(err)
	}

	want := `If you think cryptography is the answer to your problem,
then you don't know what your problem is.

— PeterNeumann x08go5TLVsV7w+/Jm0XLNKQkjjZ2UVyKcfGfOVbB8vpB6WKubYYoCVS/G9Gma/yyGakrt3cmofJIwbVdBjTuf78Ufgk=
`
	if string(msg) != want {
		t.Errorf("Sign: wrong output\nhave:\n%s\nwant:\n%s", msg, want)
	}

	// Check that existing signature is replaced by new one.
	msg, err = Sign(&Note{Text: text, Sigs: []Signature{{Name: "PeterNeumann", Hash: 0xc74f20a3, Base64: "BADSIGN="}}}, signer)
	if err != nil {
		t.Fatal(err)
	}
	if string(msg) != want {
		t.Errorf("Sign replacing signature: wrong output\nhave:\n%s\nwant:\n%s", msg, want)
	}

	// Check various bad inputs.
	_, err = Sign(&Note{Text: "abc"}, signer)
	if err == nil || err.Error() != "malformed note" {
		t.Fatalf("Sign with short text: %v, want malformed note error", err)
	}

	_, err = Sign(&Note{Text: text, Sigs: []Signature{{Name: "a+b", Base64: "ABCD"}}})
	if err == nil || err.Error() != "malformed note" {
		t.Fatalf("Sign with bad name: %v, want malformed note error", err)
	}

	_, err = Sign(&Note{Text: text, Sigs: []Signature{{Name: "PeterNeumann", Hash: 0xc74f20a3, Base64: "BADHASH="}}})
	if err == nil || err.Error() != "malformed note" {
		t.Fatalf("Sign with bad pre-filled signature: %v, want malformed note error", err)
	}

	_, err = Sign(&Note{Text: text}, &badSigner{signer})
	if err == nil || err.Error() != "invalid signer" {
		t.Fatalf("Sign with bad signer: %v, want invalid signer error", err)
	}

	_, err = Sign(&Note{Text: text}, &errSigner{signer})
	if err != errSurprise {
		t.Fatalf("Sign with failing signer: %v, want errSurprise", err)
	}
}

func TestNotaryList(t *testing.T) {
	peterKey := "PeterNeumann+c74f20a3+ARpc2QcUPDhMQegwxbzhKqiBfsVkmqq/LDE4izWy10TW"
	peterVerifier, err := NewVerifier(peterKey)
	if err != nil {
		t.Fatal(err)
	}

	enochKey := "EnochRoot+af0cfe78+ATtqJ7zOtqQtYqOo0CpvDXNlMhV3HeJDpjrASKGLWdop"
	enochVerifier, err := NewVerifier(enochKey)
	if err != nil {
		t.Fatal(err)
	}

	list := NotaryList(peterVerifier, enochVerifier, enochVerifier)
	v, err := list.Verifier("PeterNeumann", 0xc74f20a3)
	if v != peterVerifier || err != nil {
		t.Fatalf("list.Verifier(peter) = %v, %v, want %v, nil", v, err, peterVerifier)
	}
	v, err = list.Verifier("PeterNeumann", 0xc74f20a4)
	if v != nil || err == nil || err.Error() != "unknown notary key PeterNeumann+c74f20a4" {
		t.Fatalf("list.Verifier(peter bad hash) = %v, %v, want nil, unknown notary key error", v, err)
	}

	v, err = list.Verifier("PeterNeuman", 0xc74f20a3)
	if v != nil || err == nil || err.Error() != "unknown notary key PeterNeuman+c74f20a3" {
		t.Fatalf("list.Verifier(peter bad name) = %v, %v, want nil, unknown notary key error", v, err)
	}
	v, err = list.Verifier("EnochRoot", 0xaf0cfe78)
	if v != nil || err == nil || err.Error() != "ambiguous notary key EnochRoot+af0cfe78" {
		t.Fatalf("list.Verifier(enoch) = %v, %v, want nil, ambiguous notary key error", v, err)
	}
}

type badSigner struct {
	Signer
}

func (b *badSigner) Name() string {
	return "bad name"
}

var errSurprise = errors.New("surprise!")

type errSigner struct {
	Signer
}

func (e *errSigner) Sign([]byte) ([]byte, error) {
	return nil, errSurprise
}

func fmtSig(s Signature) string {
	return fmt.Sprintf("{%q %#08x %s}", s.Name, s.Hash, s.Base64)
}

func TestOpen(t *testing.T) {
	peterKey := "PeterNeumann+c74f20a3+ARpc2QcUPDhMQegwxbzhKqiBfsVkmqq/LDE4izWy10TW"
	peterVerifier, err := NewVerifier(peterKey)
	if err != nil {
		t.Fatal(err)
	}

	enochKey := "EnochRoot+af0cfe78+ATtqJ7zOtqQtYqOo0CpvDXNlMhV3HeJDpjrASKGLWdop"
	enochVerifier, err := NewVerifier(enochKey)
	if err != nil {
		t.Fatal(err)
	}

	text := `If you think cryptography is the answer to your problem,
then you don't know what your problem is.
`
	peterSig := "— PeterNeumann x08go5TLVsV7w+/Jm0XLNKQkjjZ2UVyKcfGfOVbB8vpB6WKubYYoCVS/G9Gma/yyGakrt3cmofJIwbVdBjTuf78Ufgk=\n"
	enochSig := "— EnochRoot rwz+eN7GD/vGRFArBVH5u1DgYP9NWzFasdqVlsv+SpkcVyxK32//7Q/P6+JH3hx1JMPE6fNm5ZbhvpAYgZDn6gYKvAw=\n"

	peter := Signature{"PeterNeumann", 0xc74f20a3, "x08go5TLVsV7w+/Jm0XLNKQkjjZ2UVyKcfGfOVbB8vpB6WKubYYoCVS/G9Gma/yyGakrt3cmofJIwbVdBjTuf78Ufgk="}
	enoch := Signature{"EnochRoot", 0xaf0cfe78, "rwz+eN7GD/vGRFArBVH5u1DgYP9NWzFasdqVlsv+SpkcVyxK32//7Q/P6+JH3hx1JMPE6fNm5ZbhvpAYgZDn6gYKvAw="}

	// Check one signature verified, one not.
	n, err := Open([]byte(text+"\n"+peterSig+enochSig), NotaryList(peterVerifier))
	if err != nil {
		t.Fatal(err)
	}
	if n.Text != text {
		t.Errorf("n.Text = %q, want %q", n.Text, text)
	}
	if len(n.Sigs) != 1 || n.Sigs[0] != peter {
		t.Errorf("n.Sigs:\nhave %v\nwant %v", n.Sigs, []Signature{peter})
	}
	if len(n.UnverifiedSigs) != 1 || n.UnverifiedSigs[0] != enoch {
		t.Errorf("n.UnverifiedSigs:\nhave %v\nwant %v", n.Sigs, []Signature{peter})
	}

	// Check both verified.
	n, err = Open([]byte(text+"\n"+peterSig+enochSig), NotaryList(peterVerifier, enochVerifier))
	if err != nil {
		t.Fatal(err)
	}
	if len(n.Sigs) != 2 || n.Sigs[0] != peter || n.Sigs[1] != enoch {
		t.Errorf("n.Sigs:\nhave %v\nwant %v", n.Sigs, []Signature{peter, enoch})
	}
	if len(n.UnverifiedSigs) != 0 {
		t.Errorf("n.UnverifiedSigs:\nhave %v\nwant %v", n.Sigs, []Signature{})
	}

	// Check both unverified.
	n, err = Open([]byte(text+"\n"+peterSig+enochSig), NotaryList())
	if n != nil || err == nil {
		t.Fatalf("Open unverified = %v, %v, want nil, error", n, err)
	}
	e, ok := err.(*UnverifiedNoteError)
	if !ok {
		t.Fatalf("Open unverified: err is %T, want *UnverifiedNoteError", err)
	}
	if err.Error() != "note has no verifiable signatures" {
		t.Fatalf("Open unverified: err.Error() = %q, want %q", err.Error(), "note has no verifiable signatures")
	}

	n = e.Note
	if n == nil {
		t.Fatalf("Open unverified: missing note in UnverifiedNoteError")
	}
	if len(n.Sigs) != 0 {
		t.Errorf("n.Sigs:\nhave %v\nwant %v", n.Sigs, []Signature{})
	}
	if len(n.UnverifiedSigs) != 2 || n.UnverifiedSigs[0] != peter || n.UnverifiedSigs[1] != enoch {
		t.Errorf("n.UnverifiedSigs:\nhave %v\nwant %v", n.Sigs, []Signature{peter, enoch})
	}

	// Check duplicated verifier.
	_, err = Open([]byte(text+"\n"+enochSig), NotaryList(enochVerifier, peterVerifier, enochVerifier))
	if err == nil || err.Error() != "ambiguous notary key EnochRoot+af0cfe78" {
		t.Fatalf("Open with duplicated verifier: err=%v, want ambiguous notary key", err)
	}

	// Check unused duplicated verifier.
	_, err = Open([]byte(text+"\n"+peterSig), NotaryList(enochVerifier, peterVerifier, enochVerifier))
	if err != nil {
		t.Fatal(err)
	}

	// Check too many signatures.
	n, err = Open([]byte(text+"\n"+strings.Repeat(peterSig, 101)), NotaryList(peterVerifier))
	if n != nil || err == nil || err.Error() != "malformed note" {
		t.Fatalf("Open too many verified signatures = %v, %v, want nil, malformed note error", n, err)
	}
	n, err = Open([]byte(text+"\n"+strings.Repeat(peterSig, 101)), NotaryList())
	if n != nil || err == nil || err.Error() != "malformed note" {
		t.Fatalf("Open too many verified signatures = %v, %v, want nil, malformed note error", n, err)
	}

	// Invalid signature.
	n, err = Open([]byte(text+"\n"+peterSig[:60]+"ABCD"+peterSig[60:]), NotaryList(peterVerifier))
	if n != nil || err == nil || err.Error() != "invalid signature for notary key PeterNeumann+c74f20a3" {
		t.Fatalf("Open too many verified signatures = %v, %v, want nil, invalid signature error", n, err)
	}

	// Invalid encoded message syntax.
	badMsgs := []string{
		text,
		text + "\n",
		text + "\n" + peterSig[:len(peterSig)-1],
		"\x01" + text + "\n" + peterSig,
		"\xff" + text + "\n" + peterSig,
		text + "\n" + "— Bad Name x08go5TLVsV7w+/Jm0XLNKQkjjZ2UVyKcfGfOVbB8vpB6WKubYYoCVS/G9Gma/yyGakrt3cmofJIwbVdBjTuf78Ufgk=\n",
		text + "\n" + peterSig + "Unexpected line.\n",
	}
	for _, msg := range badMsgs {
		n, err := Open([]byte(msg), NotaryList(peterVerifier))
		if n != nil || err == nil || err.Error() != "malformed note" {
			t.Fatalf("Open bad msg = %v, %v, want nil, malformed note error\nmsg:\n%s", n, err, msg)
		}
	}
}

func BenchmarkOpen(b *testing.B) {
	vkey := "PeterNeumann+c74f20a3+ARpc2QcUPDhMQegwxbzhKqiBfsVkmqq/LDE4izWy10TW"
	msg := []byte("If you think cryptography is the answer to your problem,\n" +
		"then you don't know what your problem is.\n" +
		"\n" +
		"— PeterNeumann x08go5TLVsV7w+/Jm0XLNKQkjjZ2UVyKcfGfOVbB8vpB6WKubYYoCVS/G9Gma/yyGakrt3cmofJIwbVdBjTuf78Ufgk=\n")

	verifier, err := NewVerifier(vkey)
	if err != nil {
		b.Fatal(err)
	}
	notaries := NotaryList(verifier)
	notaries0 := NotaryList()

	// Try with 0 signatures and 1 signature so we can tell how much each signature adds.

	b.Run("Sig0", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := Open(msg, notaries0)
			e, ok := err.(*UnverifiedNoteError)
			if !ok {
				b.Fatal("expected UnverifiedNoteError")
			}
			n := e.Note
			if len(n.Sigs) != 0 || len(n.UnverifiedSigs) != 1 {
				b.Fatal("wrong signature count")
			}
		}
	})

	b.Run("Sig1", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			n, err := Open(msg, notaries)
			if err != nil {
				b.Fatal(err)
			}
			if len(n.Sigs) != 1 || len(n.UnverifiedSigs) != 0 {
				b.Fatal("wrong signature count")
			}
		}
	})
}
