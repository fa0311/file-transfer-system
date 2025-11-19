package main

import (
	"fmt"
	"net"
)

func NewListener(port string) (net.Listener, error) {
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on port %s: %v", port, err)
	}
	return lis, nil
}
