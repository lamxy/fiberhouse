package utils

import (
	"fmt"
	"github.com/lamxy/fiberhouse/constant"
	"io"
	"math"
	"net/http"
	"os"
)

func DownloadImg2File(url, fileName string) error {
	response, err := http.Get(url)
	if err != nil {
		return err
	}
	defer func() {
		_ = response.Body.Close()
	}()

	// 检查 HTTP 响应状态码
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", response.Status)
	}

	// 创建一个文件用于保存图片
	file, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	// 将 HTTP 响应主体（即图片数据）写入文件
	_, err = io.Copy(file, response.Body)
	if err != nil {
		return err
	}

	return nil
}

func PageNext(page, pageSize int64) (offset, limit int64) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = constant.DefaultPageSize
	}
	offset = (page - 1) * pageSize
	limit = pageSize
	return
}

func PageParams(page, pageSize int64) (offset, limit int64) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = constant.DefaultPageSize
	}
	offset = (page - 1) * pageSize
	limit = pageSize
	return
}

func Round(f float64, precision float64) float64 {
	if precision == 0 {
		return math.Round(f)
	}
	pow := math.Pow(10, precision)

	return math.Round(f*pow) / pow
}
