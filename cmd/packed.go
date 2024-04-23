package main

import "encoding/base64"

func readPackedFile(path string) []byte {
	if b, ok := packedFiles[path]; !ok {
		return nil
	} else {
		b, err := base64.StdEncoding.DecodeString(b)
		if err != nil {
			panic(err)
		}

		return b
	}
}

var packedFiles = map[string]string{
	"./data//words/english_10k.json": "",
	"./data//words/english_5k.json":  "",
	"./data//words/english_1k.json":  "",
	"./data//quotes/english.json":    "",
	"./themes/default.txt":           "",
}
