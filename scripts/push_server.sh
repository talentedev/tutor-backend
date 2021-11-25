cd /var/learnt.io/bin/releases
aws s3 cp s3://${1}/${2}/${3}/api/${4} .
chmod 755 ${4}
cd ..
rm -f api; ln -s releases/${4} api
echo Restarting services
systemctl restart learnt-api
systemctl restart nginx
