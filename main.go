package main

import (
	"dd/utils"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kingpin"
	"github.com/gosuri/uiprogress"
)

var (
	drive    string
	savePath string
	progress bool
	BS       int
	Seek     int64
	Skip     int64
)

func main() {
	kingpin.CommandLine.Help = `
基于golang实现的window镜像工具(windd)

注意:

请使用管理员权限运行本程序

基本使用:dd.exe --if=\\.\PhysicalDrive0 --of=D:\\test.img --bs=64 --progress

	`
	kingpin.CommandLine.Name = "dd.exe"
	kingpin.Flag("if", `输入块设备或文件。像--if=\\.\PhysicalDrive0`).Required().StringVar(&drive)
	kingpin.Flag("of", `源块设备或文件。像--of=D:\\test.img`).Required().StringVar(&savePath)
	kingpin.Flag("bs", `块大小(输入和输出)，覆盖ibs和obs(以MB为单位)默认为16 最大为64MB 像--bs=16`).Default("16").IntVar(&BS)
	kingpin.Flag("skip", `从输出文件跳过前n个字节再开始写入`).Default("0").Int64Var(&Skip)
	kingpin.Flag("seek", `从输入文件的第n个字节开始读取`).Default("0").Int64Var(&Seek)

	kingpin.Flag("progress", `显示进度条`).Default("false").BoolVar(&progress)
	kingpin.Parse()
	var driveTotal int64
	var err error
	if BS > 64 {
		BS = 64
	}
	//判断是否为物理磁盘
	if strings.Contains(strings.ToLower(drive), strings.ToLower(`\\.\PhysicalDrive`)) {
		driveTotal, err = utils.GetDiskTotal(drive)
	} else {
		driveTotal, err = utils.GetLogicalTotal(drive)
	}
	if driveTotal == 0 {
		println("获取磁盘大小失败", err)
		os.Exit(1)
	} else {
		println("磁盘大小为：", driveTotal, utils.FormatFileSize(uint64(driveTotal)))
	}
	imageSize := utils.GetLogicalUsed(filepath.VolumeName(savePath))
	if imageSize == 0 {
		println("获取存储磁盘空间失败")
		os.Exit(1)
	}
	println("存储磁盘可用空间为：", imageSize, utils.FormatFileSize(uint64(imageSize)))
	if imageSize <= driveTotal {
		fmt.Println("存储磁盘空间不足,请检测磁盘空间")
		os.Exit(1)
	}
	glbalDrive, offset := utils.GetGobalDisk(drive)
	sourceFile, err := os.Open(glbalDrive)
	if err != nil {
		fmt.Println("磁盘读取失败")
		os.Exit(1)
	}
	defer sourceFile.Close()
	sourceFile.Seek(offset+Skip, io.SeekStart)
	// 创建一个字节切片来存储前1024个字节
	buffer := make([]byte, 1024*1024*BS)
	var destFile *os.File
	if utils.IsExist(savePath) {
		destFile, err = os.Open(savePath)
	} else {
		// 创建目标文件
		destFile, err = os.Create(savePath)
	}
	if err != nil {
		fmt.Println("出现异常,存储设备不存在")
		os.Exit(1)
	}
	_, err = destFile.Seek(Seek, io.SeekStart)
	if err != nil {
		fmt.Println("输出文件跳过失败", err)
		os.Exit(1)
	}
	defer destFile.Close()
	SaveedSize := int64(0)
	var bar *uiprogress.Bar
	if progress {
		uiprogress.Start() // start rendering
		bar = uiprogress.AddBar(100)
		// prepend the current step to the bar
		bar.PrependFunc(func(b *uiprogress.Bar) string {
			return fmt.Sprintf("%s/%s  %v%%", utils.FormatFileSize(uint64(SaveedSize)), utils.FormatFileSize(uint64(driveTotal)), int(SaveedSize*100/driveTotal))
		})
	}

	for {
		// 从源文件读取数据到缓冲区
		bytesRead, err := sourceFile.Read(buffer)
		if err != nil {
			if err == io.EOF {
				SaveedSize += int64(bytesRead)
				break // 文件读取完毕，退出循环
			}

			fmt.Printf("磁盘读取失败: %v\n", err)
			os.Exit(1)
		}
		if offset != 0 && (SaveedSize+int64(bytesRead)) >= driveTotal {
			// 将缓冲区的数据写入到目标文件
			_, err = destFile.Write(buffer[:driveTotal-SaveedSize])
			if err != nil {
				fmt.Printf("磁盘读取失败: %v\n", err)
				os.Exit(1)
			}
			break
		} else {
			// 将缓冲区的数据写入到目标文件
			_, err = destFile.Write(buffer[:bytesRead])
			if err != nil {
				fmt.Printf("磁盘读取失败: %v\n", err)
				os.Exit(1)
			}
			SaveedSize += int64(bytesRead)
		}

		if progress {
			bar.Set(int(SaveedSize * 100 / driveTotal))
		}

	}
	// 确保内容已写入硬盘
	destFile.Sync()
	if progress {
		bar.Set(100)
	}
	fmt.Println("磁盘存储完成")
}
