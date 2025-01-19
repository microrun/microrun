SUMMARY = "Custom Go-based system management runtime process"
LICENSE = "MIT"
LIC_FILES_CHKSUM = "file://${COMMON_LICENSE_DIR}/MIT;md5=0835ade698e0bcf8506ecda2f7b4f302"

GO_IMPORT = "github.com/microrun/microrun/userspace/runtimed"

SRC_URI = "git://github.com/microrun/microrun.git;protocol=https;branch=main"
SRCREV = "${AUTOREV}"

inherit go-static

do_install() {
    install -d ${D}${base_sbindir}
    install -m 0755 ${B}/bin/runtimed ${D}${base_sbindir}/init
}

FILES:${PN} += "${base_sbindir}/init"