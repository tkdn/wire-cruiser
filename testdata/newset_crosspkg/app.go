package nscross

import "example.com/nscross/service"

type App struct {
	S *service.Service
}

func NewApp(s *service.Service) *App {
	return &App{S: s}
}
