// n9e-2kai: 负责人/人员管理路由
package router

import (
	"github.com/ccfos/nightingale/v6/models"

	"github.com/gin-gonic/gin"
	"github.com/toolkits/pkg/ginx"
)

// configCloudStaffRoutes 配置负责人管理路由
func (rt *Router) configCloudStaffRoutes(pages *gin.RouterGroup) {
	pages.GET("/cloud-management/staff", rt.auth(), rt.user(), rt.perm("/cloud-management"), rt.staffGets)
	pages.GET("/cloud-management/staff/names", rt.auth(), rt.user(), rt.perm("/cloud-management"), rt.staffNames)
	pages.POST("/cloud-management/staff", rt.auth(), rt.user(), rt.perm("/cloud-management"), rt.staffAdd)
	pages.PUT("/cloud-management/staff/:id", rt.auth(), rt.user(), rt.perm("/cloud-management"), rt.staffUpdate)
	pages.DELETE("/cloud-management/staff", rt.auth(), rt.user(), rt.perm("/cloud-management"), rt.staffDel)
}

// staffGets 获取负责人列表
func (rt *Router) staffGets(c *gin.Context) {
	query := c.Query("query")
	limit := ginx.QueryInt(c, "limit", 100)
	offset := ginx.QueryInt(c, "offset", 0)

	list, total, err := models.CloudStaffGetAll(rt.Ctx, query, limit, offset)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	ginx.NewRender(c).Data(gin.H{
		"list":  list,
		"total": total,
	}, nil)
}

// staffNames 获取所有负责人姓名（用于下拉选择）
func (rt *Router) staffNames(c *gin.Context) {
	names, err := models.CloudStaffNames(rt.Ctx)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	ginx.NewRender(c).Data(names, nil)
}

// staffAdd 添加负责人
func (rt *Router) staffAdd(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	username := c.GetString("username")
	staff := &models.CloudStaff{
		Name:     req.Name,
		CreateBy: username,
		UpdateBy: username,
	}

	if err := staff.Add(rt.Ctx); err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	ginx.NewRender(c).Data(gin.H{"id": staff.Id}, nil)
}

// staffUpdate 更新负责人
func (rt *Router) staffUpdate(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")

	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	staff, err := models.CloudStaffGet(rt.Ctx, id)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	username := c.GetString("username")
	staff.Name = req.Name
	staff.UpdateBy = username

	if err := staff.Update(rt.Ctx); err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	ginx.NewRender(c).Message("")
}

// staffDel 删除负责人
func (rt *Router) staffDel(c *gin.Context) {
	var req struct {
		Ids []int64 `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	if err := models.CloudStaffDel(rt.Ctx, req.Ids); err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	ginx.NewRender(c).Message("")
}
