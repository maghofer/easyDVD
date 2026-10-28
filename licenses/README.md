# Build/runtime notices

These files preserve notices for Go and C runtime support which may be linked
into easyDVD itself. They do not constitute a corresponding-source clearance
for the separately bundled DVD tools.

`cdrtools-COPYRIGHTS.txt` additionally preserves copyright/license comments from
the source directories used by the source-built mkisofs helper, including its
support libraries. Its original archive and all local changes are supplied in
the toolchain source companion. This notice supplements the GPL and library
notices in `notices/authoring` in the portable package.

- `Go-LICENSE.txt`: verbatim LICENSE from the pinned Go distribution.
- `GCC-RUNTIME-EXCEPTION-3.1.txt`: GCC runtime exception, version 3.1.
  Copied verbatim from the runtime license included with the pinned innoextract
  distribution; the exception text is a general GNU license document.
- `GPL-3.0.txt`: GNU GPLv3 text accompanying that runtime exception.
- MinGW notices: the upstream COPYING, AUTHORS, DISCLAIMER, DISCLAIMER.PD and
  the complete COPYING.MinGW-w64 / COPYING.MinGW-w64-runtime notice directories.
  Their retrieval commit and hashes are recorded in validation/BUILD-NOTICES.json.

GCC's runtime exception permits eligible compiled programs to use independent
license terms. easyDVD is built with the unmodified GCC toolchain and standard
Go/cgo compilation. Go dependency notices are copied separately into the
package's `notices/go` tree, including the C/OpenGL/GLFW sources carried by those
modules and the Go font notice. The companion source ZIP preserves the full
Go module archives. Windows system DLLs are supplied by Windows, not this ZIP.

Primary references: [GCC runtime exception](https://www.gnu.org/licenses/gcc-exception-3.1.html),
[MinGW notices](https://github.com/mingw-w64/mingw-w64),
[Go license](https://go.dev/LICENSE).
