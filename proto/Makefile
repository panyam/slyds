buf: clean
	buf dep update
	buf generate
	goimports -w `find ../gen | grep "\.go"`

clean:
	rm -Rf ../gen
	rm -f buf.lock
