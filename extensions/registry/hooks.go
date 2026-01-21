package registry

import (
	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"github.com/gin-gonic/gin"
)

type Extension interface {
	Name() string
	Init(ctx *ctx.Context) error
	RegisterRoutes(router *gin.Engine) error
}

var extensions []Extension

func Register(ext Extension) {
	extensions = append(extensions, ext)
}

func InitAll(ctx *ctx.Context) error {
	for _, ext := range extensions {
		if err := ext.Init(ctx); err != nil {
			return err
		}
	}
	return nil
}

func RegisterAllRoutes(router *gin.Engine) error {
	for _, ext := range extensions {
		if err := ext.RegisterRoutes(router); err != nil {
			return err
		}
	}
	return nil
}