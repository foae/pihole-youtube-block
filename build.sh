#!/usr/bin/env bash

# Left for backwards compatibility; please use the proper-platform-suffixed binary
go build main.go
mv main bin/ytblock

# Left for backwards compatibility; please use the proper-platform-suffixed binary
env GOOS=linux GOARCH=arm GOARM=5 go build main.go
mv main bin/ytblock-rpi

package="github.com/foae/pihole-youtube-block"
package_name="ytblock"
package_split=(${package//\// })
platforms=("windows/amd64" "windows/386" "linux/amd64" "linux/386" "linux/arm" "linux/arm64" "darwin/amd64" "darwin/386" "netbsd/amd64" "netbsd/386")

for platform in "${platforms[@]}"; do
  platform_split=(${platform//\// })
  GOOS=${platform_split[0]}
  GOARCH=${platform_split[1]}
  output_name=$package_name'-'$GOOS'-'$GOARCH
  if [ $GOOS = "windows" ]; then
    output_name+='.exe'
  fi

  env GOOS=$GOOS GOARCH=$GOARCH go build -o ./bin/$output_name $package
  if [ $? -ne 0 ]; then
    echo 'An error has occurred! Aborting the script execution...'
    exit 1
  fi
  echo "Finished building [$package_name] for [$platform]"
done
