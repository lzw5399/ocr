/**
 * @Author: lzw5399
 * @Date: 2020/9/30 23:24
 * @Desc: ocr related functionality
 */
package controller

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"bank-ocr/global"
	"bank-ocr/global/response"
	"bank-ocr/model/request"
	"bank-ocr/service"

	"github.com/gin-gonic/gin"
)

const (
	INVALID_IMG_TYPE_MSG = "invalid file or unsupported file type. Only support .jpg .jpeg .png .gif .tiff, please double check!"
	INVALID_BASE64_MSG   = "invalid or unsupported BASE64 file type, please double check!"
)

func ScanFile(c *gin.Context) {
	var r request.FileFormRequest
	if err := c.ShouldBind(&r); err != nil {
		response.Failed(c, http.StatusBadRequest)
		return
	}

	upload, err := r.File.Open()
	if err != nil {
		response.FailWithMsg(c, http.StatusBadRequest, INVALID_IMG_TYPE_MSG)
		return
	}
	defer upload.Close()

	// 确保file类型是支持的image类型
	valid, contentType, err := service.EnsureFileType(upload)
	if err != nil || !valid {
		response.FailWithMsg(c, http.StatusBadRequest, INVALID_IMG_TYPE_MSG)
		return
	}

	// 灰度化
	img, err := service.GrayImage(upload)
	if err != nil {
		global.BANK_LOGGER.Error(err)
		response.Failed(c, http.StatusInternalServerError)
		return
	}

	// 根据hocrMode类型返回ocr最终的值
	global.BANK_LOGGER.Debug("start ocring")
	text, err := service.GetTextFromImage(img, contentType, r.OcrBase)

	if err != nil {
		global.BANK_LOGGER.Error(err)
		response.Failed(c, http.StatusInternalServerError)
		return
	}
	global.BANK_LOGGER.Debug("end ocring ok")

	if r.HOCRMode {
		response.OkWithPureData(c, text)
	} else {
		response.OkWithData(c, text)
	}
}

func ScanCropFile(c *gin.Context) {
	var r request.FileWithPixelPointRequest
	if err := c.ShouldBind(&r); err != nil {
		response.Failed(c, http.StatusBadRequest)
		return
	}

	// 绑定像素点 (gin的bind不能绑定formdata的对象数组)
	b := c.PostFormArray("matrixPixels")
	if len(b) == 0 {
		response.FailWithMsg(c, http.StatusBadRequest, "matrixPixels is empty.")
		return
	}
	var matrixPixels []request.MatrixPixel
	err := json.Unmarshal([]byte(b[0]), &matrixPixels)
	if err != nil {
		response.FailWithMsg(c, http.StatusBadRequest, "matrixPixels is not legally required json.")
		return
	}
	r.MatrixPixels = matrixPixels

	// 获取file
	upload, err := r.File.Open()
	if err != nil {
		response.FailWithMsg(c, http.StatusBadRequest, INVALID_IMG_TYPE_MSG)
		return
	}
	defer upload.Close()

	// 确保file类型是支持的image类型
	valid, contentType, err := service.EnsureFileType(upload)
	if err != nil || !valid {
		response.FailWithMsg(c, http.StatusBadRequest, INVALID_IMG_TYPE_MSG)
		return
	}

	// 针对像素坐标点进行裁剪并灰度化
	imgs, err := service.CropAndGrayImage(upload, r.MatrixPixels)
	if err != nil {
		global.BANK_LOGGER.Error(err)
		response.Failed(c, http.StatusInternalServerError)
		return
	}

	// 裁剪之后的图片进行ocr识别
	global.BANK_LOGGER.Debug("start ocring")
	texts, err := service.OcrTextFromImages(imgs, contentType, r.OcrBase)
	if err != nil {
		global.BANK_LOGGER.Error(err)
		response.Failed(c, http.StatusInternalServerError)
		return
	}
	global.BANK_LOGGER.Debug("end ocring ok")

	if r.HOCRMode {
		response.OkWithPureData(c, texts)
	} else {
		response.OkWithData(c, texts)
	}
}

func Base64(c *gin.Context) {
	var r request.Base64Request
	if err := c.ShouldBind(&r); err != nil {
		response.Failed(c, http.StatusBadRequest)
		return
	}

	// 确保是合法的content-type
	base64Str, isPdf, _, err := service.EnsureContentType(r.Base64)
	if err != nil {
		response.FailWithMsg(c, http.StatusBadRequest, INVALID_BASE64_MSG)
		return
	}

	// 一般的image返回的结果是string, pdf的话会是[]string
	var finalData interface{}
	if isPdf {
		// pdf分页转成png，并获取[][]byte
		start := time.Now()
		global.BANK_LOGGER.Info("[START]pdf转成图片并识别成[][]byte开始")
		bufArray, err := service.PdfToImgsThenGetBytes(base64Str)
		if err != nil {
			global.BANK_LOGGER.Error(err)
			response.Failed(c, http.StatusInternalServerError)
			return
		}
		global.BANK_LOGGER.Info("[END]pdf转成图片并识别成[][]byte耗时", time.Since(start))

		// ocr识别
		start = time.Now()
		global.BANK_LOGGER.Info("[START]OCR识别[][]byte成[]string开始")
		var texts []string
		for _, buf := range bufArray {
			text, err := service.OcrTextFromBytes(r.OcrBase, buf)
			if err != nil {
				global.BANK_LOGGER.Error(err)
				response.Failed(c, http.StatusInternalServerError)
				return
			}
			texts = append(texts, text)
		}
		global.BANK_LOGGER.Info("[END]OCR识别[][]byte成[]string耗时", time.Since(start))
		finalData = texts
	} else {
		// 图片的base64走的逻辑
		buf, err := base64.StdEncoding.DecodeString(base64Str)
		if err != nil {
			response.FailWithMsg(c, http.StatusBadRequest, INVALID_BASE64_MSG)
			return
		}

		// ocr识别[]byte
		finalData, err = service.OcrTextFromBytes(r.OcrBase, buf)
		if err != nil {
			global.BANK_LOGGER.Error(err)
			response.Failed(c, http.StatusInternalServerError)
			return
		}
	}

	if r.HOCRMode {
		response.OkWithPureData(c, finalData)
	} else {
		response.OkWithData(c, finalData)
	}
}

func ScanCropBase64(c *gin.Context) {
	var r request.Base64WithPixelPointRequest
	if err := c.ShouldBind(&r); err != nil {
		response.Failed(c, http.StatusBadRequest)
		return
	}

	// 确保是合法的content-type
	base64Str, isPdf, contentType, err := service.EnsureContentType(r.Base64)
	if err != nil || isPdf {
		response.FailWithMsg(c, http.StatusBadRequest, INVALID_BASE64_MSG)
		return
	}

	buf, err := base64.StdEncoding.DecodeString(base64Str)
	if err != nil {
		response.FailWithMsg(c, http.StatusBadRequest, INVALID_BASE64_MSG)
		return
	}

	upload := bytes.NewReader(buf)
	// 针对像素坐标点进行裁剪并灰度化
	imgs, err := service.CropAndGrayImage(upload, r.MatrixPixels)
	if err != nil {
		global.BANK_LOGGER.Error(err)
		response.Failed(c, http.StatusInternalServerError)
		return
	}

	// 裁剪之后的图片进行ocr识别
	global.BANK_LOGGER.Debug("start ocring")
	texts, err := service.OcrTextFromImages(imgs, contentType, r.OcrBase)
	if err != nil {
		global.BANK_LOGGER.Error(err)
		response.Failed(c, http.StatusInternalServerError)
		return
	}
	global.BANK_LOGGER.Debug("end ocring ok")

	if r.HOCRMode {
		response.OkWithPureData(c, texts)
	} else {
		response.OkWithData(c, texts)
	}
}
