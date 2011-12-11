all:
	6g diskmoose.go
	6l -o diskmoose diskmoose.6

gcc:
	gccgo diskmoose.go -o diskmoose

clean:
	rm -f *.6 diskmoose
