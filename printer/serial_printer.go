// for serial printer, compatiable with windows virtual serial USB printer
package printer

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tarm/serial"
	"github.com/xiaohao0576/odoo-epos/raster"
	"github.com/xiaohao0576/odoo-epos/transformer"
)

type SerialPrinter struct {
	paperWidth        int                         // 纸张宽度
	marginBottom      int                         // 下边距
	cutCommand        []byte                      // 切纸命令
	cashDrawerCommand []byte                      // 钱箱命令
	serialConfig      string                      // 串口配置字符串
	fd                *serial.Port                // 打印机文件描述符
	transformer       transformer.TransformerFunc // 用于转换图像的转换器
}

func (p *SerialPrinter) String() string {
	return fmt.Sprintf("SerialPrinter{serialConfig: %s, paperWidth: %d, marginBottom: %d}", p.serialConfig, p.paperWidth, p.marginBottom)
}

// serialConfig格式: "COM1,baud=115200,databits=8,parity=N,stopbits=1"
func (p *SerialPrinter) Open() error {
	if p.serialConfig == "" {
		return os.ErrInvalid
	}
	// 默认参数（适配大多数80mm热敏USB虚拟串口打印机）
	port := "COM1"
	baud := 115200
	databits := 8
	parity := serial.ParityNone
	stopbits := serial.Stop1

	parts := strings.Split(p.serialConfig, ",")
	if len(parts) > 0 {
		port = parts[0]
	}
	for _, part := range parts[1:] {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key, val := strings.ToLower(strings.TrimSpace(kv[0])), strings.TrimSpace(kv[1])
		switch key {
		case "baud":
			if v, err := strconv.Atoi(val); err == nil {
				baud = v
			}
		case "databits":
			if v, err := strconv.Atoi(val); err == nil {
				databits = v
			}
		case "parity":
			switch strings.ToUpper(val) {
			case "N":
				parity = serial.ParityNone
			case "O":
				parity = serial.ParityOdd
			case "E":
				parity = serial.ParityEven
			}
		case "stopbits":
			if val == "2" {
				stopbits = serial.Stop2
			}
		}
	}

	c := &serial.Config{
		Name:     port,
		Baud:     baud,
		Size:     byte(databits),
		Parity:   parity,
		StopBits: stopbits,
	}
	s, err := serial.OpenPort(c)
	if err != nil {
		return err
	}
	p.fd = s
	return nil
}

func (p *SerialPrinter) OpenCashBox() error {
	err := p.Reset()
	if err != nil {
		return fmt.Errorf("failed to reset printer: %w", err)
	}
	defer p.fd.Close()
	p.fd.Write(p.cashDrawerCommand)
	return nil
}

func (p *SerialPrinter) PrintRasterImage(img *raster.RasterImage) error {
	img = p.transformer(img) // 使用转换器转换图像
	if img == nil {
		return nil // 如果转换器返回 nil，表示不需要打印图像
	}

	err := p.Reset()
	if err != nil {
		return fmt.Errorf("failed to reset printer: %w", err)
	}
	defer p.fd.Close()

	for _, page := range img.CutPages() {
		page.AutoMarginLeft(p.paperWidth)
		page.AddMarginBottom(p.marginBottom)
		p.fd.Write(page.ToEscPosRasterCommand(1024))
		p.fd.Write(p.cutCommand)    // 切纸命令
		time.Sleep(1 * time.Second) // 等待打印机处理
	}

	return nil
}

func (p *SerialPrinter) Reset() error {
	p.Open()
	_, err := p.fd.Write([]byte{0x1B, 0x40}) // 初始化打印机
	if err != nil {
		p.fd.Close()
		p.fd = nil
		return err
	}
	return nil
}

func (p *SerialPrinter) PrintRaw(data []byte) error {
	err := p.Open()
	if err != nil {
		return fmt.Errorf("failed to open printer: %w", err)
	}
	defer p.fd.Close()
	if len(data) == 0 {
		return fmt.Errorf("no data to print")
	}
	if _, err := p.fd.Write(data); err != nil {
		return fmt.Errorf("failed to write data to printer: %w", err)
	}
	time.Sleep(1 * time.Second) // 等待打印机处理
	return nil
}
