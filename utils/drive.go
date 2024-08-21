package utils

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

type STORAGE_DEVICE_NUMBER struct {
	DeviceType      uint32
	DeviceNumber    uint32
	PartitionNumber uint32
}
type PartitionStyle uint32

const (
	IOCTL_DISK_GET_DRIVE_LAYOUT_EX = 0x00070050
	IOCTL_DISK_GET_DRIVE_GEOMETRY  = 0x00070000 // 控制代码，用于获取磁盘几何信息

	FILE_DEVICE_MASS_STORAGE        uint32 = 0x0000002d
	IOCTL_STORAGE_BASE              uint32 = FILE_DEVICE_MASS_STORAGE
	FILE_ANY_ACCESS                 uint16 = 0
	FILE_SPECIAL_ACCESS             uint16 = FILE_ANY_ACCESS
	FILE_READ_ACCESS                uint16 = 0x0001
	FILE_WRITE_ACCESS               uint16 = 0x0002
	METHOD_BUFFERED                 uint8  = 0
	METHOD_IN_DIRECT                uint8  = 1
	METHOD_OUT_DIRECT               uint8  = 2
	METHOD_NEITHER                  uint8  = 3
	IOCTL_STORAGE_GET_DEVICE_NUMBER uint32 = (IOCTL_STORAGE_BASE << 16) | uint32(FILE_ANY_ACCESS<<14) | uint32(0x0420<<2) | uint32(METHOD_BUFFERED)

	PartitionStyleMbr PartitionStyle = 0
	PartitionStyleGpt PartitionStyle = 1
	PartitionStyleRaw PartitionStyle = 2
	FILE_DEVICE_DISK  uint32         = 0x7
)

type DiskGeometry struct {
	Cylinders         int64
	MediaType         uint32
	TracksPerCylinder uint32
	SectorsPerTrack   uint32
	BytesPerSector    uint32
}

// 获取磁盘句柄
func GetDiskHandleByNum(diskName string) (syscall.Handle, error) {
	disk, _ := syscall.UTF16PtrFromString(diskName)
	handle, err := syscall.CreateFile(
		disk,
		syscall.GENERIC_READ,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE,
		nil,
		syscall.OPEN_EXISTING,
		0,
		0,
	)
	return handle, err
}

// 获取磁盘大小
func GetDiskTotal(diskName string) (int64, error) {
	// 构建磁盘路径
	disk, err := syscall.UTF16PtrFromString(diskName)
	if err != nil {
		return 0, err
	}

	// 打开磁盘句柄
	handle, err := syscall.CreateFile(
		disk,
		syscall.GENERIC_READ,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE,
		nil,
		syscall.OPEN_EXISTING,
		0,
		0,
	)
	if err != nil {
		return 0, err
	}
	defer syscall.CloseHandle(handle)

	// 准备用于获取磁盘几何信息的结构体
	var diskGeometry DiskGeometry
	var bytesReturned uint32

	// 调用 DeviceIoControl 函数获取磁盘几何信息
	err = syscall.DeviceIoControl(
		handle,
		IOCTL_DISK_GET_DRIVE_GEOMETRY,
		nil,
		0,
		(*byte)(unsafe.Pointer(&diskGeometry)),
		uint32(unsafe.Sizeof(diskGeometry)),
		&bytesReturned,
		nil,
	)
	if err != nil {
		return 0, err
	}

	// 计算磁盘总量
	diskSize := int64(diskGeometry.Cylinders) *
		int64(diskGeometry.TracksPerCylinder) *
		int64(diskGeometry.SectorsPerTrack) *
		int64(diskGeometry.BytesPerSector)

	return diskSize, nil
}

// 获取Bitlocker对应的磁盘及其盘符对应的偏移量
func GetBitlockerDrive(drive string) (string, int64) {
	dinfo, err := GetDriveBasicInfo(drive)
	if err != nil {
		return "", 0
	}
	DeviceNumber := dinfo.DeviceNumber
	disk, err := GetDiskHandleByNum(fmt.Sprintf(`\\.\PhysicalDrive%d`, DeviceNumber))
	if err != nil {
		if err == syscall.ERROR_FILE_NOT_FOUND {
			// 物理磁盘号不存在，结束枚举
			return "", 0
		}
	}
	defer syscall.CloseHandle(disk)

	data, err := getAllPartitionInfo(disk)
	if err != nil {
		return "", 0
	}
	header := (*DRIVE_LAYOUT_INFORMATION_EX_HEADER)(unsafe.Pointer(&data[0]))
	next := data[int(unsafe.Sizeof(*header)):]
	entryOffset := GetSizeOf_DRIVE_LAYOUT_INFORMATION()
	entryData := next[entryOffset:]
	entrySize := unsafe.Sizeof(PARTITION_INFORMATION_EX{})
	for i := 0; i < int(header.PartitionCount); i++ {
		if len(entryData) < int(entrySize) {
			break
		}
		partitionEntry := (*PARTITION_INFORMATION_EX)(unsafe.Pointer(&entryData[0]))
		entryData = entryData[entrySize:]
		if partitionEntry.DeviceNumber == int32(dinfo.PartitionNumber) {
			return fmt.Sprintf(`\\.\PhysicalDrive%d`, DeviceNumber), partitionEntry.StartingOffset
		}
	}
	return "", 0
}

// 获取目标读取文件及其偏移量
func GetGobalDisk(drive string) (string, int64) {
	//检测drive是否可读
	sourceFile, err := os.Open(drive)
	if err != nil {
		fmt.Println("磁盘打开失败", err)
		os.Exit(1)
	}
	defer sourceFile.Close()
	buffer := make([]byte, 1024)
	// 从源文件读取数据到缓冲区
	_, err = sourceFile.Read(buffer)
	if err != nil {
		if strings.Contains(err.Error(), "BitLocker") {
			fmt.Println("检测到盘符为bitlocker加密盘,正在切换镜像模式")
			return GetBitlockerDrive(drive)
		}
		fmt.Printf("磁盘读取失败: %v\n", err)
		os.Exit(1)
	}
	return drive, 0

}
