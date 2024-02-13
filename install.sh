#!/bin/bash

red='\033[0;31m'
green='\033[0;32m'
yellow='\033[0;33m'
plain='\033[0m'

cur_dir=$(pwd)

# check root
[[ $EUID -ne 0 ]] && echo -e "${red}Fatal error: ${plain} Please run this script with root privilege \n " && exit 1

# Check OS and set release variable
if [[ -f /etc/os-release ]]; then
    source /etc/os-release
    release=$ID
elif [[ -f /usr/lib/os-release ]]; then
    source /usr/lib/os-release
    release=$ID
else
    echo "Failed to check the system OS, please contact the author!" >&2
    exit 1
fi
echo "The OS release is: $release"

arch=$(arch)

if [[ $arch == "x86_64" || $arch == "x64" || $arch == "amd64" ]]; then
    arch="amd64"
elif [[ $arch == "aarch64" || $arch == "arm64" ]]; then
    arch="arm64"
elif [[ $arch == "s390x" ]]; then
    arch="s390x"
else
    arch="amd64"
    echo -e "${red} Failed to check system arch, will use default arch: ${arch}${plain}"
fi

echo "arch: ${arch}"

if [ $(getconf WORD_BIT) != '32' ] && [ $(getconf LONG_BIT) != '64' ]; then
    echo "s-ui dosen't support 32-bit(x86) system, please use 64 bit operating system(x86_64) instead, if there is something wrong, please get in touch with me!"
    exit -1
fi

os_version=""
os_version=$(grep -i version_id /etc/os-release | cut -d \" -f2 | cut -d . -f1)

if [[ "${release}" == "centos" ]]; then
    if [[ ${os_version} -lt 8 ]]; then
        echo -e "${red} Please use CentOS 8 or higher ${plain}\n" && exit 1
    fi
elif [[ "${release}" ==  "ubuntu" ]]; then
    if [[ ${os_version} -lt 20 ]]; then
        echo -e "${red}please use Ubuntu 20 or higher version! ${plain}\n" && exit 1
    fi

elif [[ "${release}" == "fedora" ]]; then
    if [[ ${os_version} -lt 36 ]]; then
        echo -e "${red}please use Fedora 36 or higher version! ${plain}\n" && exit 1
    fi

elif [[ "${release}" == "debian" ]]; then
    if [[ ${os_version} -lt 10 ]]; then
        echo -e "${red} Please use Debian 10 or higher ${plain}\n" && exit 1
    fi
else
    echo -e "${red}Failed to check the OS version, please contact the author!${plain}" && exit 1
fi


install_base() {
    if [[ "${release}" == "centos" ]] || [[ "${release}" == "fedora" ]] ; then
        yum install wget curl tar unzip jq -y
    else
        apt install wget curl tar unzip jq -y
    fi
}

install_s-ui() {
    cd /tmp/

    if [ $# == 0 ]; then
        last_version=$(curl -Ls "https://api.github.com/repos/alireza0/s-ui/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
        if [[ ! -n "$last_version" ]]; then
            echo -e "${red}Failed to fetch s-ui version, it maybe due to Github API restrictions, please try it later${plain}"
            exit 1
        fi
        echo -e "Got s-ui latest version: ${last_version}, beginning the installation..."
        wget -N --no-check-certificate -O /tmp/s-ui-linux-${arch}.tar.gz https://github.com/alireza0/s-ui/releases/download/${last_version}/s-ui-linux-${arch}.tar.gz
        if [[ $? -ne 0 ]]; then
            echo -e "${red}Dowanloading s-ui failed, please be sure that your server can access Github ${plain}"
            exit 1
        fi
    else
        last_version=$1
        url="https://github.com/alireza0/s-ui/releases/download/${last_version}/s-ui-linux-${arch}.tar.gz"
        echo -e "Begining to install s-ui v$1"
        wget -N --no-check-certificate -O /tmp/s-ui-linux-${arch}.tar.gz ${url}
        if [[ $? -ne 0 ]]; then
            echo -e "${red}dowanload s-ui v$1 failed,please check the verison exists${plain}"
            exit 1
        fi
    fi

    if [[ -e /usr/local/s-ui/ ]]; then
        systemctl stop s-ui
        systemctl stop sing-box
    fi

    tar zxvf s-ui-linux-${arch}.tar.gz
    rm s-ui-linux-${arch}.tar.gz -f
    chmod +x s-ui/sui s-ui/bin/sing-box s-ui/bin/runSingbox.sh
    cp -rf s-ui /usr/local/
    cp -f s-ui/*.service /etc/systemd/system/
    rm -rf s-ui

    systemctl daemon-reload
    systemctl enable s-ui  --now
    systemctl enable sing-box --now

    echo -e "${green}s-ui v${last_version}${plain} installation finished, it is up and running now..."
    echo -e ""
    echo -e "s-ui usages: "
    echo -e "----------------------------------------------"
    echo -e "s-ui              - Enter     Run S-UI"
    echo -e "----------------------------------------------"
}

echo -e "${green}Excuting...${plain}"
install_base
install_s-ui $1