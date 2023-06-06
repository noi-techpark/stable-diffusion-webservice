// SPDX-FileCopyrightText: NOI Techpark <digital@noi.bz.it>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package utils

import (
	"crypto/rand"
	"fmt"
	"time"
)

func GetRandomToken() (string, error) {
	buf := make([]byte, 16)
	_, err := rand.Read(buf)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%0x", buf), nil
}

func Log(msg string) {
	fmt.Printf("%s: %s\n", time.Now().UTC(), msg)
}
