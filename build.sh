#!/usr/bin/env bash


package_name="gitaclean"
platforms=("windows/amd64" "windows/386" "darwin/amd64" "linux/amd64" "linux/386")

version=$(git describe --tags --always --dirty)
echo "Building version:" ${version}

for platform in "${platforms[@]}"
do
    platform_split=(${platform//\// })
    GOOS=${platform_split[0]}
    GOARCH=${platform_split[1]}
    output_name=$package_name'-'$GOOS'-'$GOARCH
    if [ $GOOS = "windows" ]; then
        output_name+='.exe'
    fi

    env GOOS=$GOOS GOARCH=$GOARCH go build -o $output_name -ldflags="-X main.version=${version}" $package
    if [ $? -ne 0 ]; then
        echo 'An error has occurred! Aborting the script execution...'
        exit 1
    fi
done