// @APIVersion 1.0.0
// @Title Capture Order
// @Description Capture Order
// @Contact shanepec@microsoft.com
// @TermsOfServiceUrl
// @License MIT
// @LicenseUrl
package routers

import (
	"captureorderfd/controllers"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/context"
	"github.com/astaxie/beego/plugins/cors"
)

func init() {
	ns := beego.NewNamespace("/v1",
		beego.NSNamespace("/order",
			beego.NSInclude(
				&controllers.OrderController{},
			),
		),
	)
	beego.AddNamespace(ns)
	beego.Get("/healthz", func(ctx *context.Context) {
		ctx.Output.Body([]byte("i'm alive!"))
	})
	beego.InsertFilter("*", beego.BeforeRouter, cors.Allow(&cors.Options{
		AllowAllOrigins: true,
		AllowMethods:    []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:    []string{"Origin", "Authorization", "Access-Control-Allow-Origin"},
		ExposeHeaders:   []string{"Content-Length", "Access-Control-Allow-Origin"},
	}))
}
