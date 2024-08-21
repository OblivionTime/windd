package utils

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type GUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

type DRIVE_LAYOUT_INFORMATION_GPT struct {
	DiskId               GUID
	StartingUsableOffset uint64
	UsableLength         uint64
	MaxPartitionCount    uint32
}

type PARTITION_INFORMATION_MBR struct {
	PartitionType       byte
	BootIndicator       bool
	RecognizedPartition bool
	HiddenSectors       uint32
	PartitionId         GUID
}

type PARTITION_INFORMATION_GPT struct {
	PartitionType GUID
	PartitionId   GUID
	Attributes    uint64
	Name          [36]uint16
}

type PARTITION_INFORMATION_EX struct {
	PartitionStyle   PartitionStyle
	StartingOffset   int64
	PartitionLength  int64
	DeviceNumber     int32
	RewritePartition bool
	Rev01            bool
	Rev02            bool
	Rev03            bool
	PartitionInfo    [112]byte
}
type DRIVE_LAYOUT_INFORMATION_MBR struct {
	Signature uint32
	CheckSum  uint32
}

type DRIVE_LAYOUT_INFORMATION_EX_HEADER struct {
	PartitionStyle PartitionStyle
	PartitionCount uint32
}

func GetSizeOf_DRIVE_LAYOUT_INFORMATION() int {
	a := unsafe.Sizeof(DRIVE_LAYOUT_INFORMATION_GPT{})
	b := unsafe.Sizeof(DRIVE_LAYOUT_INFORMATION_MBR{})
	if a > b {
		return int(a)
	} else {
		return int(b)
	}
}
func getAllPartitionInfo(diskHandle syscall.Handle) ([]byte, error) {
	var bytesReturned uint32
	buffer := make([]byte, 4096)
	err := syscall.DeviceIoControl(diskHandle, IOCTL_DISK_GET_DRIVE_LAYOUT_EX, nil, 0, &buffer[0], uint32(len(buffer)), &bytesReturned, nil)
	if err != nil {
		return nil, err
	}
	return buffer, nil
}

// 获取逻辑盘符的基本信息
func GetDriveBasicInfo(drive string) (STORAGE_DEVICE_NUMBER, error) {
	var disk_num STORAGE_DEVICE_NUMBER
	var err error
	filepath, _ := syscall.UTF16PtrFromString(drive)
	handle, err := syscall.CreateFile(
		filepath,
		syscall.GENERIC_READ,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE,
		nil,
		syscall.OPEN_EXISTING,
		0,
		0)
	if ^uintptr(0) == uintptr(handle) {
		fmt.Printf("CreateFile() failed, errmsg = %s\n", err.Error())
		return disk_num, nil
	}
	var size uint32 = uint32(unsafe.Sizeof(disk_num))
	var ret_size uint32 = 0
	var outbuf *byte = (*byte)(unsafe.Pointer(&disk_num))
	syscall.DeviceIoControl(
		handle,
		IOCTL_STORAGE_GET_DEVICE_NUMBER,
		nil, 0,
		outbuf, size,
		&ret_size, nil)
	syscall.CloseHandle(handle)
	return disk_num, nil
}

// 获取逻辑盘符大小
func GetLogicalTotal(drive string) (int64, error) {
	dinfo, err := GetDriveBasicInfo(drive)
	if err != nil {
		return 0, err
	}
	DeviceNumber := dinfo.DeviceNumber
	disk, err := GetDiskHandleByNum(fmt.Sprintf(`\\.\PhysicalDrive%d`, DeviceNumber))
	if err != nil {
		if err == syscall.ERROR_FILE_NOT_FOUND {
			// 物理磁盘号不存在，结束枚举
			return 0, err
		}
	}
	defer syscall.CloseHandle(disk)

	data, err := getAllPartitionInfo(disk)
	if err != nil {
		fmt.Errorf("Failed to get partition info: %v\n", err)
		return 0, err
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
			return partitionEntry.PartitionLength, nil
		}
	}
	return 0, fmt.Errorf("未获取到逻辑盘符大小")
}

// 获取逻辑盘符的使用量
func GetLogicalUsed(drive string) int64 {
	var freeBytes uint64
	windows.GetDiskFreeSpaceEx(windows.StringToUTF16Ptr(drive), &freeBytes, nil, nil)
	return int64(freeBytes)
}
