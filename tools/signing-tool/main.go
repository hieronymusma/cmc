// Copyright(c) 2021 Fraunhofer AISEC
// Fraunhofer-Gesellschaft zur Foerderung der angewandten Forschung e.V.
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the License); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an AS IS BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"io/ioutil"
	"strings"

	log "github.com/sirupsen/logrus"
)

type Serializer interface {
	Sign(data []byte, keys []crypto.PrivateKey, x5cs [][]*x509.Certificate) ([]byte, error)
}

// TODO mode verify und output gleich noch mit einbauen!!

func main() {
	log.SetLevel(log.TraceLevel)

	metadata := flag.String("data", "", "Path to metadata as JSON or CBOR to be signed")
	keyFiles := flag.String("keys", "", "Paths to keys in PEM format to be used for signing, as a comma-separated list")
	x5cFiles := flag.String("x5cs", "", "Paths to PEM encoded x509 certificate chains, as a comma-separated list")
	format := flag.String("format", "JSON", "Format of the metadata (JSON or CBOR)")
	outputFile := flag.String("out", "", "Path to the output file to save signed metadata")
	flag.Parse()

	if *metadata == "" {
		log.Error("metadata file not specified (--data)")
		flag.Usage()
		return
	}
	if *keyFiles == "" {
		log.Error("key file(s) not specified (--keys)")
		flag.Usage()
		return
	}
	if *x5cFiles == "" {
		log.Error("certificate chain file(s) not specified (--x5cs)")
		flag.Usage()
		return
	}
	if *outputFile == "" {
		log.Error("output file not specified (--out)")
		flag.Usage()
		return
	}

	// Load metadata
	data, err := ioutil.ReadFile(*metadata)
	if err != nil {
		log.Fatalf("failed to read metadata file %v", *metadata)
	}

	// Load keys
	s1 := strings.Split(*keyFiles, ",")
	keys := make([]crypto.PrivateKey, 0)
	for _, keyFile := range s1 {
		keyPem, err := ioutil.ReadFile(keyFile)
		if err != nil {
			log.Fatalf("failed to read key file %v", err)
		}

		block, _ := pem.Decode(keyPem)
		if block == nil || block.Type != "EC PRIVATE KEY" {
			log.Fatal("failed to decode PEM block containing private key")
		}

		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			log.Fatal("Failed to parse private key")
		}

		keys = append(keys, key)
	}
	if len(keys) == 0 {
		log.Fatal("No valid keys specified")
	}
	log.Tracef("Read %v private keys", len(keys))

	// Load certificate chains
	s2 := strings.Split(*x5cFiles, ",")
	certs := make([][]*x509.Certificate, 0)
	for _, pemFile := range s2 {
		certsPem, err := ioutil.ReadFile(pemFile)
		if err != nil {
			log.Fatalf("failed to read certificate(s) file %v", err)
		}

		c, err := loadCerts(certsPem)
		if err != nil {
			log.Fatalf("Failed to load certificates: %v", err)
		}

		certs = append(certs, c)
	}
	if len(certs) == 0 {
		log.Fatal("No valid certificates specified")
	}
	log.Tracef("Read %v certificates", len(certs))

	if len(certs) != len(keys) {
		log.Fatalf("Number of certificates (%v) does not match number of keys (%v)", certs, keys)
	}

	// Create serializer based on specified format
	var s Serializer
	if strings.EqualFold(*format, "json") {
		s = JsonSerializer{}
	} else if strings.EqualFold(*format, "cbor") {
		s = CborSerializer{}
	} else {
		log.Fatalf("Serializer %v not supported (only JSON and CBOR are supported)", *format)
	}

	// Sign metadata
	signedData, err := s.Sign(data, keys, certs)
	if err != nil {
		log.Fatalf("failed to sign data: %v", err)
	}

	err = ioutil.WriteFile(*outputFile, signedData, 0644)
	if err != nil {
		log.Fatalf("failed to write output file: %v", err)
	}
}

// TODO use from Attestation Report Module or define generic module
func loadCerts(data []byte) ([]*x509.Certificate, error) {
	certs := make([]*x509.Certificate, 0)
	input := data

	for block, rest := pem.Decode(input); block != nil; block, rest = pem.Decode(rest) {

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse x509 Certificate: %v", err)
		}
		certs = append(certs, cert)
	}
	return certs, nil
}