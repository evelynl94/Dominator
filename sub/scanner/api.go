package scanner

import (
	"fmt"
	"github.com/Symantec/Dominator/lib/objectcache"
	"github.com/Symantec/Dominator/sub/fsrateio"
	"io"
	"regexp"
	"time"
)

type Configuration struct {
	FsScanContext *fsrateio.FsRateContext
	ExclusionList []*regexp.Regexp
}

func (configuration *Configuration) SetExclusionList(reList []string) error {
	return configuration.setExclusionList(reList)
}

type FileSystemHistory struct {
	fileSystem         *FileSystem
	scanCount          uint64
	generationCount    uint64
	timeOfLastScan     time.Time
	durationOfLastScan time.Duration
	timeOfLastChange   time.Time
}

func (fsh *FileSystemHistory) Update(newFS *FileSystem) {
	fsh.update(newFS)
}

func (fsh *FileSystemHistory) FileSystem() *FileSystem {
	return fsh.fileSystem
}

func (fsh *FileSystemHistory) GenerationCount() uint64 {
	return fsh.generationCount
}

func (fsh FileSystemHistory) String() string {
	return fmt.Sprintf("GenerationCount=%d\n", fsh.generationCount)
}

func (fsh *FileSystemHistory) WriteHtml(writer io.Writer) {
	fsh.writeHtml(writer)
}

type RegularInodeTable map[uint64]*RegularInode
type SymlinkInodeTable map[uint64]*SymlinkInode
type InodeTable map[uint64]*Inode
type InodeList map[uint64]bool

type FileSystem struct {
	configuration      *Configuration
	rootDirectoryName  string
	RegularInodeTable  RegularInodeTable
	SymlinkInodeTable  SymlinkInodeTable
	InodeTable         InodeTable // This excludes directories.
	DirectoryInodeList InodeList
	TotalDataBytes     uint64
	HashCount          uint64
	ObjectCache        objectcache.ObjectCache
	Dev                uint64
	Directory
}

func ScanFileSystem(rootDirectoryName string, cacheDirectoryName string,
	configuration *Configuration) (*FileSystem, error) {
	return scanFileSystem(rootDirectoryName, cacheDirectoryName, configuration,
		nil)
}

func (fs *FileSystem) Configuration() *Configuration {
	return fs.configuration
}

func (fs *FileSystem) RebuildPointers() {
	fs.rebuildPointers()
}

func (fs *FileSystem) String() string {
	return fmt.Sprintf("Tree: %d inodes, total file size: %s, number of hashes: %d\nObjectCache: %d objects\n",
		len(fs.RegularInodeTable)+len(fs.SymlinkInodeTable)+len(fs.InodeTable)+
			len(fs.DirectoryInodeList),
		fsrateio.FormatBytes(fs.TotalDataBytes),
		fs.HashCount,
		len(fs.ObjectCache))
}

func (fs *FileSystem) WriteHtml(writer io.Writer) {
	fs.writeHtml(writer)
}

func (fs *FileSystem) DebugWrite(w io.Writer, prefix string) error {
	return fs.debugWrite(w, prefix)
}

type Directory struct {
	Name            string
	RegularFileList []*RegularFile
	SymlinkList     []*Symlink
	FileList        []*File
	DirectoryList   []*Directory
	Mode            uint32
	Uid             uint32
	Gid             uint32
}

func (directory *Directory) String() string {
	return directory.Name
}

func (directory *Directory) DebugWrite(w io.Writer, prefix string) error {
	return directory.debugWrite(w, prefix)
}

type RegularInode struct {
	Mode             uint32
	Uid              uint32
	Gid              uint32
	MtimeNanoSeconds int32
	MtimeSeconds     int64
	Size             uint64
	Hash             [64]byte
}

type RegularFile struct {
	Name        string
	InodeNumber uint64
	inode       *RegularInode
}

func (file *RegularFile) String() string {
	return file.Name
}

func (file *RegularFile) DebugWrite(w io.Writer, prefix string) error {
	return file.debugWrite(w, prefix)
}

type SymlinkInode struct {
	Uid     uint32
	Gid     uint32
	Symlink string
}

type Symlink struct {
	Name        string
	InodeNumber uint64
	inode       *SymlinkInode
}

func (symlink *Symlink) DebugWrite(w io.Writer, prefix string) error {
	return symlink.debugWrite(w, prefix)
}

type Inode struct {
	Mode             uint32
	Uid              uint32
	Gid              uint32
	MtimeNanoSeconds int32
	MtimeSeconds     int64
	Rdev             uint64
}

type File struct {
	Name        string
	InodeNumber uint64
	inode       *Inode
}

func (file *File) String() string {
	return file.Name
}

func (file *File) DebugWrite(w io.Writer, prefix string) error {
	return file.debugWrite(w, prefix)
}

func Compare(left *FileSystem, right *FileSystem, logWriter io.Writer) bool {
	return compare(left, right, logWriter)
}

func StartScannerDaemon(rootDirectoryName string, cacheDirectoryName string,
	configuration *Configuration) chan *FileSystem {
	return startScannerDaemon(rootDirectoryName, cacheDirectoryName,
		configuration)
}
