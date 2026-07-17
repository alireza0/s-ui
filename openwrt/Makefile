include $(TOPDIR)/rules.mk

PKG_NAME:=sui
PKG_VERSION:=1.0.0
PKG_RELEASE:=1

PKG_SOURCE_PROTO:=git
PKG_SOURCE_URL:=https://github.com/alireza0/s-ui.git
PKG_SOURCE_VERSION:=HEAD
PKG_SOURCE_DATE:=2024-07-17

PKG_MAINTAINER:=Jules
PKG_LICENSE:=GPL-3.0-or-later

PKG_BUILD_DEPENDS:=golang/host nodejs/host
PKG_BUILD_PARALLEL:=1

GO_PKG:=github.com/alireza0/s-ui
GO_PKG_BUILD_PKG:=github.com/alireza0/s-ui

include $(INCLUDE_DIR)/package.mk
include ../../lang/golang/golang-package.mk

define Package/sui
  SECTION:=net
  CATEGORY:=Network
  TITLE:=s-ui
  URL:=https://github.com/alireza0/s-ui
  DEPENDS:=$(GO_ARCH_DEPENDS) +sing-box
endef

define Package/sui/description
  s-ui panel
endef

# Build frontend and backend
define Build/Compile
	(cd $(PKG_BUILD_DIR)/frontend && npm i && npm run build)
	mkdir -p $(PKG_BUILD_DIR)/web/html
	rm -rf $(PKG_BUILD_DIR)/web/html/*
	cp -R $(PKG_BUILD_DIR)/frontend/dist/* $(PKG_BUILD_DIR)/web/html/
	$(call GoPackage/Build/Compile)
endef

define Package/sui/install
	$(INSTALL_DIR) $(1)/usr/bin
	$(INSTALL_BIN) $(GO_PKG_BUILD_BIN_DIR)/s-ui $(1)/usr/bin/sui
	$(INSTALL_DIR) $(1)/usr/share/s-ui/web
	$(CP) $(PKG_BUILD_DIR)/frontend/dist/* $(1)/usr/share/s-ui/web/
	$(INSTALL_DIR) $(1)/etc/init.d
	$(INSTALL_BIN) $(PKG_BUILD_DIR)/openwrt/s-ui.init $(1)/etc/init.d/s-ui
endef

$(eval $(call GoPackage,sui))
$(eval $(call BuildPackage,sui))
