#!/bin/bash

#set -ex

function printHelp() {
                echo $(basename $0)" options:";
                echo "    -A <Architectures to use> # Compiling to ${ARCHS} now, examples: linux/amd64,linux/arm/v7,linux/arm/v6,linux/arm64"
                if [ ${FLAG_NOCACHE} -gt 0 ]
                then
                        echo "    -C # Do not use cache"
                else 
                        echo "    -C # Use cache" 
                fi
                if [ ${FLAG_UPLOADIMAGES} -gt 0 ]
                then
                        echo "    -U # Do not upload images - the default is upload the images to the registry"
                else 
                        echo "    -U # Upload images - the default is not to upload the images to the registry"
                fi
                if [ ${FLAG_UPLOADMANIFEST} -gt 0 ] 
                then
                        echo "    -M # Do not upload manifest - the default is upload the manifest to the registry"
                else
                        echo "    -U # Upload manifest - the default is not to upload the manifest to the registry"
                fi
                if [ ${FLAG_USESQUASH} -gt 0 ] 
                then
                        echo "    -S # Do not squash images - the default is to squash the images"
                else
                        echo "    -S # Squash images - the default is not to squash the images"
                fi
                echo "    -B <build tag to use> # Default is today's date with seconds UTC";
                echo "    -T <additional build tag to use> # The whole build tag will be added to the -B or the default";
                echo "    -I <add this to the image name>"
                return;
}

BUILD_TAG=$(date -u "+%Y%m%d%H%M%S")
REPOSITORY_NAME="registry.gitlab.com/arm-research/smarter/smarter-device-manager/"
IMAGE_NAME="smarter-device-manager"
DIRECTORY_TO_RUN=.

ARCHS="linux/arm64"

# Variable defaults
FLAG_UPLOADIMAGES=0
FLAG_USESQUASH=0
FLAG_UPLOADMANIFEST=1
ADDITIONAL_TAG=""
ADDITIONAL_IMAGE_NAME=""
PUSH_OPTION=""
FLAG_NOCACHE=0

while getopts hA:B:MST:UC name
do
        case $name in
        h)
                printHelp;
                exit 0;;
        A)
                ARCHS="$OPTARG";;
        C)
                [ ${FLAG_NOCACHE} -gt 0 ] && FLAG_NOCACHE=0;
                [ ${FLAG_NOCACHE} -eq 0 ] && FLAG_NOCACHE=1;
                ;;
        U)
                [ ${FLAG_UPLOADIMAGES} -gt 0 ] && FLAG_UPLOADIMAGES=0;
                [ ${FLAG_UPLOADIMAGES} -eq 0 ] && FLAG_UPLOADIMAGES=1;
                ;;
        M)
                [ ${FLAG_UPLOADMANIFEST} -gt 0 ] && FLAG_UPLOADMANIFEST=0;
                [ ${FLAG_UPLOADMANIFEST} -eq 0 ] && FLAG_UPLOADMANIFEST=1;
                ;;
        S)
                [ ${FLAG_USESQUASH} -gt 0 ] && FLAG_USESQUASH=0;
                [ ${FLAG_USESQUASH} -eq 0 ] && FLAG_USESQUASH=1;
                ;;
        B)
                BUILD_TAG="$OPTARG";;
        T)
                ADDITIONAL_TAG="$OPTARG";;
        I)
                ADDITIONAL_IMAGE_NAME="$OPTARG";;
        *)
                printHelp;
                exit 0;
                ;;
        esac
done

# We need docker client to support manifest for multiarch, try so see if we have it enabled
if [ ${FLAG_UPLOADMANIFEST} -gt 0 ]
then
        if [ -e ~/.docker/config.json ]
        then
                grep -i "experimental.*:.*enabled" ~/.docker/config.json 2>/dev/null || sed -i -e 's/^{/{\n    "experimental":"enabled",/' ~/.docker/config.json
        else
                mkdir -p ~/.docker
                cat <<EOF  > ~/.docker/config.json
{
        "experimental":"enabled"
}
EOF
        fi
fi

if [ $FLAG_NOCACHE -gt 0 ]
then
        CACHE_OPTION="--no-cache"
else
        CACHE_OPTION="" 
fi
        
if [ $FLAG_UPLOADIMAGES -gt 0 ]
then
        PUSH_OPTION="--push"
else
        PUSH_OPTION="--load"
fi
        
docker buildx build ${CACHE_OPTION}  -t "${REPOSITORY_NAME}${IMAGE_NAME}${ADDITIONAL_IMAGE_NAME}:${BUILD_TAG}" --platform=${ARCHS} ${PUSH_OPTION} .

exit 0
