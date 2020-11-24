#!/usr/bin/env bash

# Local arguments
BASEIMAGE=ubuntu:bionic
IMAGE_NAME="mqtt-test"
IMAGE_VERSION="latest" 
BUILD_CMD="docker build -f Dockerfile.build --build-arg http_proxy=$http_proxy --build-arg https_proxy=$http_proxy --build-arg"

# Network name
PREDIX_BROKER_NET=predix-edge-broker_net

# Docker run command: add '--privileged' temporarily.
RUN_CMD="docker run -d -v$(pwd):/mnt --network=${PREDIX_BROKER_NET} --rm"

# Show the script usage
usage() {
  echo "Usage:"
  echo "    ./build.sh --target={build | run | clean | net | qemu} --arch={[amd64] | x86 | aarch64 | arm} --make={[all] | test} --broker={[mqtt] | redis]"
  exit
}

# Cleanup the build
clean () {
  echo "Cleanup..."
  rm -rf *.pb.*
  rm -rf *.tar
  rm -rf *.tar.gz
  rm -rf *.zip
}

# Network create
network_create () {
  echo "Create \"${PREDIX_BROKER_NET}\""
  docker network create -d bridge $PREDIX_BROKER_NET || true
}

# QEMU installation
startQemu () {
  echo "Configure QEMU environment"
  if findmnt -rno TARGET "binfmt_misc" >/dev/null; then
    echo "Volume \"binfmt_misc\" is already mounted."
  else
    sudo sh -c "mount binfmt_misc -t binfmt_misc /proc/sys/fs/binfmt_misc"
  fi
  sudo sh -c "echo 1 > /proc/sys/fs/binfmt_misc/status"
}

stopQemu () {
  echo "Stop QEMU"
  sudo sh -c "echo 0 > /proc/sys/fs/binfmt_misc/status"
}

# Arguments parsing
parse_args () {
  for i in $@
  do
  case $i in
      -t=*|--target=*)
      ARG_TARGET="${i#*=}"
      shift # past argument=value
      ;;
      -m=*|--make=*)
      ARG_MAKE="${i#*=}"
      shift # past argument=value
      ;;
      -a=*|--arch=*)
      ARG_ARCH="${i#*=}"
      shift # past argument=value
      ;;
      *)
            # unknown option
          # usage
      ;;
  esac
  done
}

process_cmd () {
  case $ARG_TARGET in
      build)
      build_cmd $ARG_ARCH $ARG_MAKE $ARG_BRK
      ;;
      run)
      run_cmd $ARG_ARCH
      ;;
      clean)
      clean
      ;;
      net)
      network_create
      ;;
      qemu)
      startQemu
      ;;
      ~qemu)
      stopQemu
      ;;
      *)
      usage
      ;;
  esac
}

build_cmd () {
  START_QUEMU=
  STOP_QUEMU=
  case $ARG_ARCH in
      x86)
      TGT_ARCH="i386"
      AGY_LIB_ARCH="https://artifactory.build.ge.com/IWNUO/delivery/sbus/8/8.0.2.0-alpha01/lib/linux86/"
      ;;
      amd64)
      TGT_ARCH="amd64"
      AGY_LIB_ARCH="https://artifactory.build.ge.com/IWNUO/delivery/sbus/8/8.0.2.0-alpha01/lib/linux64/"
      ;;
      arm)
      TGT_ARCH="arm32v7"
      AGY_LIB_ARCH="https://artifactory.build.ge.com/IWNUO/delivery/sbus/8/8.0.2.0-alpha01/lib/linux_armv7/"
      START_QUEMU=startQemu
      STOP_QUEMU=stopQemu
      ;;
      aarch64)
      TGT_ARCH="arm64v8"
      AGY_LIB_ARCH="https://artifactory.build.ge.com/IWNUO/delivery/sbus/8/8.0.2.0-alpha01/lib/linux_armv8/"
      START_QUEMU=startQemu
      STOP_QUEMU=stopQemu
      ;;
      *)
      TGT_ARCH="amd64"
      AGY_LIB_ARCH="https://artifactory.build.ge.com/IWNUO/delivery/sbus/8/8.0.2.0-alpha01/lib/linux64/"
      echo "Unknown build architecture \"${ARG_ARCH}\", defaulting to \"${TGT_ARCH}\""
      ;;
  esac

  case $ARG_MAKE in
      all)
      MAKE_ARG="all"
      ;;
      test)
      MAKE_ARG="test"
      ;;
      *)
      MAKE_ARG="all"
      echo "Unknown make target ${ARG_MAKE}, defaulting to \"${MAKE_ARG}\""
      ;;
  esac

  case $ARG_BRK in
      mqtt)
      MSG_BROKER_ARG="--build-arg MSG_BROKER_SELECT_BUILD=mqtt"
      ;;
      redis)
      MSG_BROKER_ARG="--build-arg MSG_BROKER_SELECT_BUILD=redis"
      ;;
      *)
      MSG_BROKER_ARG="--build-arg MSG_BROKER_SELECT_BUILD=mqtt"
      echo "Unknown broker \"${ARG_BRK}\", defaulting to \"mqtt\""
      ;;
  esac

  sed -e "s|{ARCH_FROM}|$TGT_ARCH|g" docker-compose-arch.yml > docker-compose.yml
  sed -i "s|{IMAGE_VERSION}|$IMAGE_VERSION|g" docker-compose-arch.yml docker-compose.yml
  sed -e "s|ARG_FROM_BUILD|$TGT_ARCH"/"$BASEIMAGE|g" Dockerfile > Dockerfile.build
  sed -i "s|ARG_FROM|$TGT_ARCH"/"$BASEIMAGE|g" Dockerfile.build
  sed -i "s|AGY_INC_ARCH|$AGY_INC_ARCH|g" Dockerfile.build
  sed -i "s|AGY_LIB_ARCH|$AGY_LIB_ARCH|g" Dockerfile.build

  echo "Building \"${MAKE_ARG}\" for arch=\"${TGT_ARCH}\""
  
  git clone git@github.build.ge.com:Grid-Automation-Shareable/MsgBroker.git
  cp ./MsgBroker/*.cc .
  cp ./MsgBroker/*.h .
  rm -rf ./MsgBroker/

  $START_QUEMU
  $BUILD_CMD=$MAKE_ARG $MSG_BROKER_ARG -t $TGT_ARCH/$IMAGE_NAME:$IMAGE_VERSION .
  $STOP_QUEMU
  
  docker save -o $IMAGE_NAME"-"$TGT_ARCH".tar" $TGT_ARCH/$IMAGE_NAME:$IMAGE_VERSION
  tar czf $IMAGE_NAME"-"$TGT_ARCH.tar.gz $IMAGE_NAME"-"$TGT_ARCH".tar" docker-compose.yml config-agency-server.json

  zip -X -r config-agency.zip config-agency-server.json

  clean
}

run_cmd () {
  START_QUEMU=
  STOP_QUEMU=
  case $ARG_ARCH in
      x86)
      TGT_ARCH="i386"
      ;;
      amd64)
      TGT_ARCH="amd64"
      ;;
      arm)
      TGT_ARCH="arm32v7"
      START_QUEMU=startQemu
      STOP_QUEMU=stopQemu
      ;;
      aarch64)
      TGT_ARCH="arm64v8"
      START_QUEMU=startQemu
      STOP_QUEMU=stopQemu
      ;;
      *)
      TGT_ARCH="amd64"
      echo "Unknown architecture \"${ARG_ARCH}\", defaulting to \"${TGT_ARCH}\""
      ;;
  esac

  TGT_IMAGE=$TGT_ARCH/$IMAGE_NAME
  if [ ! "$(docker images -q $TGT_IMAGE)" ]; then
    echo "Container ${TGT_IMAGE} doesn't exist !!!"
    echo "  Run, first, the script with the \"build\" option."
    usage
  else
    $START_QUEMU
    $RUN_CMD ${TGT_IMAGE} ./agencyServer
    $STOP_QUEMU
  fi
}

parse_args "$@"
process_cmd