package postgres

import "github.com/samber/do"

/*
service:uses PostgreSQL
description: Stores user data and authentication tokens
technology:postgresql
proto:tcp
*/
type Connection struct{}

func NewConnection(i *do.Injector) *Connection {
	return &Connection{}
}

func (c *Connection) Query() error {
	return nil
}
