#!/usr/bin/env bash
# Reproduces the "Clang cannot link on Debian" problem and its fix, using the LLVM from the
# toolchain stage of the nginx-clang Dockerfile. Run on a Docker host, next to the Dockerfile:
#   ./repro-clang.sh [Dockerfile]
# It starts ONE container from the toolchain stage, removes libgcc-N-dev and libstdc++-N-dev,
# shows what fails, installs them again and shows that everything links.
set -uo pipefail
cd "$(dirname "$0")"
DF="${1:-Dockerfile}"
GCC=$(grep -m1 -E '^ARG GCC_MAJOR=' "$DF" | cut -d= -f2)

echo "== toolchain stage (cached after a normal build)"
docker build -q --target toolchain -f "$DF" -t clang-toolchain-repro . >/dev/null || exit 1

docker run --rm clang-toolchain-repro sh -c '
set -u
printf "int main(void){return 0;}\n" > /t.c
printf "#include <iostream>\nint main(){std::cout << \"hello from libstdc++\" << std::endl;}\n" > /t.cpp
show() { echo "-- $1"; shift; "$@" 2>&1 | head -${LINES_MAX:-6} | cut -c1-200; }
export DEBIAN_FRONTEND=noninteractive

echo; echo "== state A: the toolchain stage as built (libgcc-'"$GCC"'-dev and libstdc++-'"$GCC"'-dev installed)"
echo "C   with lld : $(clang -fuse-ld=lld /t.c -o /t 2>&1 && echo COMPILED_OK)"
echo "C++ with lld : $(clang++ -fuse-ld=lld /t.cpp -o /t 2>&1 && /t)"
clang -v /t.c -o /t 2>&1 | grep -E "Selected GCC installation"

echo; echo "== state B: remove the two development packages (what a bare Debian with only libc6-dev has)"
apt-get remove -y -qq libgcc-'"$GCC"'-dev libstdc++-'"$GCC"'-dev >/dev/null 2>&1
echo "runtime library still present (libgcc_s.so.1)       : $(ls /lib/x86_64-linux-gnu/libgcc_s.so.1 2>&1 | head -1)"
echo "linker symlink libgcc_s.so (needed by -lgcc_s)      : $(ls /usr/lib/gcc/x86_64-linux-gnu/'"$GCC"'/libgcc_s.so 2>&1 | head -1)"
echo "crtbeginS.o (startup object)                        : $(ls /usr/lib/gcc/x86_64-linux-gnu/'"$GCC"'/crtbeginS.o 2>&1 | head -1)"
echo
echo "C   default linker:"; clang /t.c -o /t 2>&1 | head -4 | cut -c1-200
echo "C   with lld      :"; clang -fuse-ld=lld /t.c -o /t 2>&1 | head -4 | cut -c1-200
echo "C++ with lld      :"; clang++ -fuse-ld=lld /t.cpp -o /t 2>&1 | head -4 | cut -c1-200
echo "Clang GCC detection:"; clang -v /t.c -o /t 2>&1 | grep -iE "Found candidate|Selected GCC|cannot|not found" | head -3

echo; echo "== state C: install them again (the fix)"
apt-get update -qq >/dev/null 2>&1
apt-get install -y -qq --no-install-recommends libgcc-'"$GCC"'-dev libstdc++-'"$GCC"'-dev >/dev/null 2>&1
echo "C   with lld : $(clang -fuse-ld=lld /t.c -o /t 2>&1 && echo COMPILED_OK)"
echo "C++ with lld : $(clang++ -fuse-ld=lld /t.cpp -o /t 2>&1 && /t)"
echo "files that come from the dev packages:"; ls /usr/lib/gcc/x86_64-linux-gnu/'"$GCC"'/ | grep -E "crtbegin|libgcc_s|libgcc.a|libstdc|crtend" | tr "\n" " "; echo
' 2>&1
docker rmi -f clang-toolchain-repro >/dev/null 2>&1
