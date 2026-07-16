package service

import "github.com/google/wire"

type Service struct{}

func NewService() *Service {
	return &Service{}
}

var Set = wire.NewSet(NewService)
