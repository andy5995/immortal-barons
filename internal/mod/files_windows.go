package mod

import (
	"os"

	"golang.org/x/sys/windows"
)

// openModFile opens a mod's file. Windows has no O_NOFOLLOW; the folder walk
// has already refused a link that leads out of the mod's folder.
func openModFile(path string) (*os.File, error) { return os.Open(path) }

// linkCount is how many names the opened file has, as the unix version
// reports it: a hard link to a file elsewhere would otherwise ride a league's
// reset order to every board.
func linkCount(f *os.File, _ os.FileInfo) uint64 {
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(windows.Handle(f.Fd()), &info); err != nil {
		return 1
	}
	return uint64(info.NumberOfLinks)
}
