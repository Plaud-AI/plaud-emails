package common

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	KeyResponseStatus = "X-Response-Status"
	KeyUserID         = "X-User-ID"
)

// Resp JSON响应
type Resp struct {
	Status int         `json:"status"`
	Msg    string      `json:"msg,omitempty"`
	Data   interface{} `json:"data,omitempty"`
	Type   string      `json:"type,omitempty"`
}

// ToJSON 转换为json
func (p *Resp) ToJSON() []byte {
	if p == nil {
		return nil
	}
	data, _ := json.Marshal(p)
	return data
}

// NewSuccessResp 构建成功的响应
func NewSuccessResp(typ string, data any) *Resp {
	return &Resp{
		Type: typ,
		Data: data,
	}
}

// NewFailResp 构建失败的响应
func NewFailResp(msg string, code int) *Resp {
	if code == 0 {
		code = -1
	}

	return &Resp{
		Type:   "error",
		Msg:    msg,
		Status: code,
	}
}

// NewFailRespFromError 将 error 转为统一失败响应
func NewFailRespFromError(err error) *Resp {
	ae := FromError(err)
	return &Resp{
		Type:   "error",
		Msg:    ae.Message,
		Status: ae.Code,
	}
}

// JSONFailResponse 设置业务响应状态并返回JSON响应, HTTP状态码为200
func JSONFailResponse(c *gin.Context, msg string, code int) {
	resp := NewFailResp(msg, code)
	JSONResponse(c, resp)
}

// JSONSuccessResponse 设置业务响应状态并返回JSON响应, HTTP状态码为200
func JSONSuccessResponse(c *gin.Context, typ string, data any) {
	resp := NewSuccessResp(typ, data)
	JSONResponse(c, resp)
}

// JSONResponse 设置业务响应状态并返回JSON响应, HTTP状态码为200
func JSONResponse(c *gin.Context, resp *Resp) {
	c.Set(KeyResponseStatus, resp.Status)
	c.JSON(http.StatusOK, resp)
}

// GetResponseStatus 获取业务响应状态
func GetResponseStatus(c *gin.Context) int {
	return c.GetInt(KeyResponseStatus)
}

// JSONResponse 设置Http状态码并返回JSON响应
func JSONResponseWithCode(c *gin.Context, code int, resp *Resp) {
	c.Set(KeyResponseStatus, resp.Status)
	c.JSON(code, resp)
}
