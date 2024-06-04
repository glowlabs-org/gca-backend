#!/bin/bash

# Exit immediately if a command exits with a non-zero status.
set -e

STAGE_DIR="stage"
CLIENTS_DIR="stage/.config/gca-data/clients"
ZIP_FILE="v2-upgrade.zip"

rm -rf ${STAGE_DIR}
mkdir -p ${CLIENTS_DIR}

echo "Building glow_monitor"
GOOS=linux GOARCH=arm GOARM=7 go build -o ${CLIENTS_DIR}/glow_monitor main.go

echo "Copying clients files"
cp halki-app monitor-sync.* monitor-udp.* ${CLIENTS_DIR}/
if [[ -f halki-password ]]; then
cp halki-password ${CLIENTS_DIR}/
fi

echo "Copying installation scripts"
cp ../gca-admin/equipment-prescan.sh ../gca-admin/equipment-setup.sh ../gca-admin/server-setup.sh ${STAGE_DIR}

echo "Creating zip file"
rm ${ZIP_FILE}
cd ${STAGE_DIR}
zip -r ../${ZIP_FILE} * .*
cd ..
echo "Release created at ${ZIP_FILE}"
