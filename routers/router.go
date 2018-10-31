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
}
