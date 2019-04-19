git clone https://github.com/tpoechtrager/osxcross.git ~/.local/share/cleave/osxcross/

cd ~/.local/share/cleave/osxcross/

cd tarballs
wget https://s3.dockerproject.org/darwin/v2/MacOSX10.11.sdk.tar.xz
cd ..

UNATTENDED=yes ./build.sh
wget https://github.com/karalabe/xgo/raw/master/docker/base/patch.tar.xz -O /tmp/patch.tar.xz
cd target/

tar -xf /tmp/patch.tar.xz -C SDK/MacOSX10.11.sdk/usr/include/c++/
