package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type NetAddress struct {
	Host string
	Port uint16
}

func (a *NetAddress) String() string {
	return fmt.Sprintf("%s:%d", a.Host, a.Port)
}

func (a *NetAddress) Set(flagVal string) error {
	paramsArr := strings.Split(flagVal, ":")
	if len(paramsArr) != 2 {
		return errors.New("contains to many arguments")
	}
	if paramsArr[1] == "" {
		return errors.New("port should be set")
	}
	port, err := strconv.Atoi(paramsArr[1])
	if err != nil {
		return errors.New("port should be a valid decimal number")
	}
	if port > 65535 { // The maximum possible port numberfor IPv4
		return errors.New("port should be not grater than 65535")
	}
	a.Host = paramsArr[0] // it migth be an empty string
	a.Port = uint16(port)
	return nil
}
