/**
 * @Author: lzw5399
 * @Date: 2020/10/2 14:50
 * @Desc:
 */
package request

type FileFormRequest struct {
	Languages string `form:"languages" json:"languages"`
	Whitelist string `form:"whitelist" json:"whitelist"`
	Format    string `form:"format" json:"format"`
	Trim      string `form:"trim" json:"trim"`
}

type FileWithPixelPointRequest struct {
	FileFormRequest
	MatrixPixels []MatrixPixel `form:"-" json:"matrixPixels"` // formdata没法绑定这种对象数组
}

// 两个像素坐标点能圈出一个矩阵
type MatrixPixel struct {
	PointA Pixel `form:"pointA" json:"pointA"`
	PointB Pixel `form:"pointB" json:"pointB"`
}
// [{ "pointA": {"x": 127, "y": 249}, "pointB": {"x": 983, "y": 309}}]
// 像素坐标点
type Pixel struct {
	X int `form:"x" json:"x"`
	Y int `form:"y" json:"y"`
}

func (r *FileWithPixelPointRequest) ToFileFormRequest() FileFormRequest {
	req := FileFormRequest{
		Languages: r.Languages,
		Whitelist: r.Whitelist,
		Format:    r.Format,
		Trim:      r.Trim,
	}

	return req
}
