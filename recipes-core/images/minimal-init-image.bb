SUMMARY = "Minimal image with custom init"
LICENSE = "MIT"

inherit core-image

# Remove package management
IMAGE_FEATURES = ""
PACKAGE_INSTALL = "\
    init-go \
    kernel-modules \
"

# Remove unnecessary packages
IMAGE_INSTALL = "${PACKAGE_INSTALL}"
EXTRA_IMAGE_FEATURES = ""

# Remove package management
BAD_RECOMMENDATIONS += "busybox-syslog"
IMAGE_FEATURES:remove = "package-management"