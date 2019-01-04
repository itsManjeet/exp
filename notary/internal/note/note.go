// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package note defines the notes signed by the Go module notary.
//
// This package is part of a DRAFT of what the Go module notary will look like.
// Do not assume the details here are final!
//
// A note is text signed by one or more notaries.
// The text should be ignored unless the note is signed by
// a trusted notary and the signature has been verified.
//
// A notary is identified by a name, typically the "host[/path]"
// giving the base URL of the notary's transparency log.
// (The transparency log allows interested auditors to check
// what the notary has signed, but it is not used directly for
// verification of signatures.)
//
// A notary signs texts using public key cryptography.
// A given notary may have multiple public keys, each
// identified by the first 32 bits of the SHA-256 hash of
// the notary name and public key.
//
// Verifying Notes
//
// A Verifier allows verification of one notary public key.
// It can report the name of the notary and the hash of the key
// and can verify a purported signature by that key.
//
// The standard implementation of a Verifier is constructed
// by NewVerifier starting from a verifier key, which is a
// plain text string of the form "<name>+<hash>+<keydata>".
//
// A Notaries allows looking up a Verifier by the combination
// of notary name and key hash.
//
// The standard implementation of a Notaries is constructed
// by NotaryList from a list of known verifiers.
//
// A Note represents a text with one or more signatures.
//
// A Signature represents a signature on a note, verified or not.
// If verified, the Name and Hash fields are set.
// If unverified, the UnverifiedName and UnverifiedHash fields are set.
//
// The Open function takes as input a signed message
// and a set of known notaries. It decodes and verifies
// the message signatures and returns a Note structure
// containing the message text and (verified or unverified) signatures.
//
// Signing Notes
//
// A Signer allows signing a text with a given key.
// It can report the name of the notary and the hash of the key
// and can sign a raw text using that key.
//
// The standard implementation of a Signer is constructed
// by NewSigner starting from an encoded signer key, which is a
// plain text string of the form "PRIVATE+KEY+<name>+<hash>+<keydata>".
// Anyone with an encoded signer key can sign messages using that key,
// so it must be kept secret. The encoding begins with the literal text
// "PRIVATE+KEY" to avoid confusion with the public verifier key.
//
// The Sign function takes as input a Note and a list of Signers
// and returns an encoded, signed message.
//
// Signed Note Format
//
// A signed note consists of a text ending in newline (U+000A),
// followed by a blank line (only a newline),
// followed by one or more signature lines of this form:
// em dash (U+2014), space (U+0020),
// notary name, space, base64-encoded signature, newline.
//
// Signed notes must be valid UTF-8 and must not contain any
// ASCII control characters (those below U+0020) other than newline.
//
// A signature is a base64 encoding of 4+n bytes.
// The four bytes are the (big-endian) key hash,
// and the remaining n bytes are the result of signing the text
// with the indicated key.
//
// Generating Keys
//
// There is only one key type, Ed25519 with algorithm identifier 1.
// New key types may be introduced in the future as needed,
// although doing so will deploying the new algorithms to all clients
// before starting to depend on them for signatures.
//
// The GenerateKey function generates and returns a new signer
// and corresponding verifier.
//
// Example
//
// Here is a well-formed signed note:
//
//	If you think cryptography is the answer to your problem,
//	then you don't know what your problem is.
//
//	— PeterNeumann x08go5TLVsV7w+/Jm0XLNKQkjjZ2UVyKcfGfOVbB8vpB6WKubYYoCVS/G9Gma/yyGakrt3cmofJIwbVdBjTuf78Ufgk=
//
// It can be constructed and displayed using:
//
//	skey := "PRIVATE+KEY+PeterNeumann+c74f20a3+AYEKFALVFGyNhPJEMzD1QIDr+Y7hfZx09iUvxdXHKDFz"
//	text := "If you think cryptography is the answer to your problem,\n" +
//		"then you don't know what your problem is.\n"
//
//	signer, err := note.NewSigner(skey)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	msg, err := note.Sign(&note.Note{Text: text}, signer)
//	if err != nil {
//		log.Fatal(err)
//	}
//	os.Stdout.Write(msg)
//
// The note's text is two lines, including the final newline,
// and the text is purportedly signed by a notary named
// "PeterNeumann". (Although notary names are canonically
// base URLs, the only syntactic requirement is that they
// not contain spaces or newlines).
//
// If Open is given access to a Notaries including the
// Verifier for this key, then it will succeed at verifiying
// the encoded message and returning the parsed Note:
//
//	vkey := "PeterNeumann+c74f20a3+ARpc2QcUPDhMQegwxbzhKqiBfsVkmqq/LDE4izWy10TW"
//	msg := []byte("If you think cryptography is the answer to your problem,\n" +
//		"then you don't know what your problem is.\n" +
//		"\n" +
//		"— PeterNeumann x08go5TLVsV7w+/Jm0XLNKQkjjZ2UVyKcfGfOVbB8vpB6WKubYYoCVS/G9Gma/yyGakrt3cmofJIwbVdBjTuf78Ufgk=\n")
//
//	verifier, err := note.NewVerifier(vkey)
//	if err != nil {
//		log.Fatal(err)
//	}
//	notaries := note.NotaryList(verifier)
//
//	n, err := note.Open([]byte(msg), notaries)
//	if err != nil {
//		log.Fatal(err)
//	}
//	fmt.Printf("%s (%08x):\n%s", n.Sigs[0].Name, n.Sigs[0].Hash, n.Text)
//
// You can add your own signature to this message by re-signing the note:
//
//	skey, vkey, err := note.GenerateKey(rand.Reader, "EnochRoot")
//	if err != nil {
//		log.Fatal(err)
//	}
//	_ = vkey // give to verifiers
//
//	me, err := note.NewSigner(skey)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	msg, err := note.Sign(n, me)
//	if err != nil {
//		log.Fatal(err)
//	}
//	os.Stdout.Write(msg)
//
// This will print a doubly-signed message, like:
//
//	If you think cryptography is the answer to your problem,
//	then you don't know what your problem is.
//
//	— PeterNeumann x08go5TLVsV7w+/Jm0XLNKQkjjZ2UVyKcfGfOVbB8vpB6WKubYYoCVS/G9Gma/yyGakrt3cmofJIwbVdBjTuf78Ufgk=
//	— EnochRoot rwz+eN7GD/vGRFArBVH5u1DgYP9NWzFasdqVlsv+SpkcVyxK32//7Q/P6+JH3hx1JMPE6fNm5ZbhvpAYgZDn6gYKvAw=
//
package note

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/ed25519"
)

// A Verifier verifies messages signed with a specific notary key.
type Verifier interface {
	// Name returns the name of the notary.
	Name() string

	// Hash returns the notary key hash.
	Hash() uint32

	// Verify reports whether sig is a valid signature of msg.
	Verify(msg, sig []byte) bool
}

// A Signer signs messages using a specific notary key.
type Signer interface {
	// Name returns the name of the notary.
	Name() string

	// Hash returns the notary key hash.
	Hash() uint32

	// Sign returns a signature for the given message.
	Sign(msg []byte) ([]byte, error)
}

// keyHash computes the key hash for the given notary name and encoded public key.
func keyHash(name string, key []byte) uint32 {
	h := sha256.New()
	h.Write([]byte(name))
	h.Write([]byte("\n"))
	h.Write(key)
	sum := h.Sum(nil)
	return binary.BigEndian.Uint32(sum)
}

var (
	errVerifierID   = errors.New("malformed verifier id")
	errVerifierAlg  = errors.New("unknown verifier algorithm")
	errVerifierHash = errors.New("invalid verifier hash")
)

const (
	algEd25519 = 1
)

// NewVerifier construct a new Verifier from an encoded verifier key.
func NewVerifier(vkey string) (Verifier, error) {
	name, vkey := chop(vkey, "+")
	hash16, key64 := chop(vkey, "+")
	hash, err1 := strconv.ParseUint(hash16, 16, 32)
	key, err2 := base64.StdEncoding.DecodeString(key64)
	if len(hash16) != 8 || err1 != nil || err2 != nil || len(key) < 1 {
		return nil, errVerifierID
	}
	if uint32(hash) != keyHash(name, key) {
		return nil, errVerifierHash
	}

	v := &verifier{
		name: name,
		hash: uint32(hash),
	}

	alg, key := key[0], key[1:]
	switch alg {
	default:
		return nil, errVerifierAlg

	case algEd25519:
		if len(key) != 32 {
			return nil, errVerifierID
		}
		v.verify = func(msg, sig []byte) bool {
			return ed25519.Verify(key, msg, sig)
		}
	}

	return v, nil
}

// chop chops s at the first instance of sep, if any,
// and returns the text before and after sep.
// If sep is not present, chop returns before is s and after is empty.
func chop(s, sep string) (before, after string) {
	i := strings.Index(s, sep)
	if i < 0 {
		return s, ""
	}
	return s[:i], s[i+len(sep):]
}

// verifier is a trivial Verifier implementation.
type verifier struct {
	name   string
	hash   uint32
	verify func([]byte, []byte) bool
}

func (v *verifier) Name() string                { return v.name }
func (v *verifier) Hash() uint32                { return v.hash }
func (v *verifier) Verify(msg, sig []byte) bool { return v.verify(msg, sig) }

// NewSigner constructs a new Signer from an encoded signer key.
func NewSigner(skey string) (Signer, error) {
	priv1, skey := chop(skey, "+")
	priv2, skey := chop(skey, "+")
	name, skey := chop(skey, "+")
	hash16, key64 := chop(skey, "+")
	hash, err1 := strconv.ParseUint(hash16, 16, 32)
	key, err2 := base64.StdEncoding.DecodeString(key64)
	if priv1 != "PRIVATE" || priv2 != "KEY" || len(hash16) != 8 || err1 != nil || err2 != nil || len(key) < 1 {
		return nil, errSignerID
	}

	// Note: hash is the hash of the public key and we have the private key.
	// Must verify hash after deriving public key.

	s := &signer{
		name: name,
		hash: uint32(hash),
	}

	var pubkey []byte

	alg, key := key[0], key[1:]
	switch alg {
	default:
		return nil, errSignerAlg

	case algEd25519:
		if len(key) != 32 {
			return nil, errSignerID
		}
		key = ed25519.NewKeyFromSeed(key)
		pubkey = append([]byte{1}, key[32:]...)
		s.sign = func(msg []byte) ([]byte, error) {
			return ed25519.Sign(key, msg), nil
		}
	}

	if uint32(hash) != keyHash(name, pubkey) {
		return nil, errSignerHash
	}

	return s, nil
}

var (
	errSignerID   = errors.New("malformed verifier id")
	errSignerAlg  = errors.New("unknown verifier algorithm")
	errSignerHash = errors.New("invalid verifier hash")
)

// signer is a trivial Signer implementation.
type signer struct {
	name string
	hash uint32
	sign func([]byte) ([]byte, error)
}

func (s *signer) Name() string                    { return s.name }
func (s *signer) Hash() uint32                    { return s.hash }
func (s *signer) Sign(msg []byte) ([]byte, error) { return s.sign(msg) }

// GenerateKey generates a signer and verifier key pair for a named notary.
// The signer key skey is private and must be kept secret.
func GenerateKey(rand io.Reader, name string) (skey, vkey string, err error) {
	pub, priv, err := ed25519.GenerateKey(rand)
	if err != nil {
		return "", "", err
	}
	pubkey := append([]byte{algEd25519}, pub...)
	privkey := append([]byte{algEd25519}, priv[:32]...)
	h := keyHash(name, pubkey)

	skey = fmt.Sprintf("PRIVATE+KEY+%s+%08x+%s", name, h, base64.StdEncoding.EncodeToString(privkey))
	vkey = fmt.Sprintf("%s+%08x+%s", name, h, base64.StdEncoding.EncodeToString(pubkey))
	return skey, vkey, nil
}

// A Notaries is a collection of known notary keys.
type Notaries interface {
	// Verifier returns the Verifier associated with the notary key
	// identified by the name and hash.
	// If the name, hash pair is unknown, Verifier should return
	// an UnknownNotaryError.
	Verifier(name string, hash uint32) (Verifier, error)
}

// An UnknownNotaryError indicates that the given notary key is not known.
// The Parse function records signatures for unknown notaries as
// unverified signatures.
type UnknownNotaryError struct {
	Name string
	Hash uint32
}

func (e *UnknownNotaryError) Error() string {
	return fmt.Sprintf("unknown notary key %s+%08x", e.Name, e.Hash)
}

// NotaryList returns a Notaries implementation that uses the given list of verifiers.
func NotaryList(list ...Verifier) Notaries {
	m := make(notaryMap)
	for _, v := range list {
		m[nameHash{v.Name(), v.Hash()}] = v
	}
	return m
}

type nameHash struct {
	name string
	hash uint32
}

type notaryMap map[nameHash]Verifier

func (m notaryMap) Verifier(name string, hash uint32) (Verifier, error) {
	v, ok := m[nameHash{name, hash}]
	if !ok {
		return nil, &UnknownNotaryError{name, hash}
	}
	return v, nil
}

// A Note is a text and a list of signatures.
type Note struct {
	Text string
	Sigs []Signature
}

// A Signature is a single signature found in a note.
type Signature struct {
	// If the signature has been verified,
	// Name and Hash give the verified name and key hash
	// for the notary key that generated the signature.
	Name string
	Hash uint32

	// If the signature has not been verified,
	// Name and Hash are empty, and instead
	// UnverifiedName and UnverifiedHash
	// give the unverified name and key hash
	// from the signature line.
	UnverifiedName string
	UnverifiedHash uint32

	// Base64 records the base64-encoded signature bytes.
	Base64 string
}

// An InvalidSignatureError indicates that the given notary key was known
// and the associated Verifier rejected the signature.
type InvalidSignatureError struct {
	Name string
	Hash uint32
}

func (e *InvalidSignatureError) Error() string {
	return fmt.Sprintf("invalid signature for notary key %s+%08x", e.Name, e.Hash)
}

var (
	errMalformedNote = errors.New("malformed note")

	sigSplit  = []byte("\n\n")
	sigPrefix = []byte("— ")
)

// Open opens and parses the message msg, checking signatures of the known notaries.
//
// For each signature in the message, Open calls known.Verifier to find a verifier.
// If known.Verifier returns a verifier and the verifier accepts the signature,
// Open records the signature in the returned note.
// If known.Verifier returns a verifier but the verifier rejects the signature,
// Open returns an InvalidSignatureError.
// If known.Verifier returns an UnknownNotaryError,
// Open records the signature in the returned note,
// but with UnverifiedName and UnverifiedHash set insetad of Name and Hash.
// If known.Verifier returns any other error, Open returns that error.
func Open(msg []byte, known Notaries) (*Note, error) {
	// Must have valid UTF-8 with no non-newline ASCII control characters.
	for i := 0; i < len(msg); {
		r, size := utf8.DecodeRune(msg[i:])
		if r < 0x20 && r != '\n' || r == utf8.RuneError && size == 1 {
			return nil, errMalformedNote
		}
		i += size
	}

	// Must end with signature block preceded by blank line.
	split := bytes.LastIndex(msg, sigSplit)
	if split < 0 {
		return nil, errMalformedNote
	}
	text, sigs := msg[:split+1], msg[split+2:]
	if len(sigs) == 0 || sigs[len(sigs)-1] != '\n' {
		return nil, errMalformedNote
	}

	n := &Note{
		Text: string(text),
	}

	var buf bytes.Buffer
	buf.Write(text)

	// Parse and verify signatures.
	for len(sigs) > 0 {
		// Pull out next signature line.
		// We know sigs[len(sigs)-1] == '\n', so IndexByte always finds one.
		i := bytes.IndexByte(sigs, '\n')
		line := sigs[:i]
		sigs = sigs[i+1:]

		if !bytes.HasPrefix(line, sigPrefix) {
			return nil, errMalformedNote
		}
		line = line[len(sigPrefix):]
		i = bytes.IndexByte(line, ' ')
		if i < 0 {
			return nil, errMalformedNote
		}
		name, b64 := string(line[:i]), string(line[i+1:])
		sig, err := base64.StdEncoding.DecodeString(b64)
		if err != nil || len(sig) < 5 {
			return nil, errMalformedNote
		}
		hash := binary.BigEndian.Uint32(sig[0:4])
		sig = sig[4:]

		v, err := known.Verifier(name, hash)
		if _, ok := err.(*UnknownNotaryError); ok {
			n.Sigs = append(n.Sigs, Signature{UnverifiedName: name, UnverifiedHash: hash, Base64: b64})
			continue
		}
		if err != nil {
			return nil, err
		}

		// The verified text is the concatenation of the actual text
		// and the name of the notary.
		// This ensures that signatures by one notary can't be
		// misinterpreted as being from a different notary,
		// even if that other notary advertises the first one's public key.
		buf.Truncate(len(text))
		buf.WriteString("\n")
		buf.WriteString(name)
		ok := v.Verify(buf.Bytes(), sig)
		if !ok {
			return nil, &InvalidSignatureError{name, hash}
		}

		n.Sigs = append(n.Sigs, Signature{Name: name, Hash: hash, Base64: b64})
	}

	// Parsed and verified all the signatures.
	return n, nil
}

// Sign signs the note with the given signers and returns the encoded message.
// The new signatures from signers are listed in the encoded message after
// the existing signatures already present in n.Sigs.
// If any signer uses the same notary key as an existing signature,
// the existing signature is elided from the output.
func Sign(n *Note, signers ...Signer) ([]byte, error) {
	var buf bytes.Buffer
	if !strings.HasSuffix(n.Text, "\n") {
		return nil, errMalformedNote
	}
	buf.WriteString(n.Text)

	// Prepare signatures.
	var sigs []Signature
	have := make(map[nameHash]bool)
	for _, s := range signers {
		name := s.Name()
		hash := s.Hash()
		have[nameHash{name, hash}] = true

		buf.Truncate(len(n.Text))
		buf.WriteString("\n")
		buf.WriteString(name)
		sig, err := s.Sign(buf.Bytes())
		if err != nil {
			return nil, err
		}

		var hbuf [4]byte
		binary.BigEndian.PutUint32(hbuf[:], hash)
		b64 := base64.StdEncoding.EncodeToString(append(hbuf[:], sig...))
		sigs = append(sigs, Signature{Name: name, Hash: hash, Base64: b64})
	}

	// Gather existing signatures.
	var old []Signature
	for _, sig := range n.Sigs {
		name, hash := sig.Name, sig.Hash
		if name == "" {
			name, hash = sig.UnverifiedName, sig.UnverifiedHash
		}
		if name == "" || strings.Contains(name, " ") || strings.Contains(name, "\n") {
			return nil, errMalformedNote
		}
		if have[nameHash{name, hash}] {
			continue
		}
		// Double-check hash against base64.
		raw, err := base64.StdEncoding.DecodeString(sig.Base64)
		if err != nil || len(raw) < 4 || binary.BigEndian.Uint32(raw) != hash {
			return nil, errMalformedNote
		}
		old = append(old, Signature{Name: name, Base64: sig.Base64})
	}

	buf.Truncate(len(n.Text))
	buf.WriteString("\n")
	for _, sig := range append(old, sigs...) {
		buf.WriteString("— ")
		buf.WriteString(sig.Name)
		buf.WriteString(" ")
		buf.WriteString(sig.Base64)
		buf.WriteString("\n")
	}

	return buf.Bytes(), nil
}
