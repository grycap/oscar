GCC=g++
#CFLAGS=-std=c++11 -DDEBUG_BINDING -g -I. -I/opt/COMPSs/Bindings/c/include -I/opt/COMPSs/Bindings/bindings-common/include
CFLAGS=-std=c++11 -g -I. -I/opt/COMPSs/Bindings/c/include -I/opt/COMPSs/Bindings/bindings-common/include

all: clean compile_master compile_worker deploy

compile_master:
	@echo "Building application for master..."
	$(GCC) -DCOMPSS_MASTER $(CFLAGS) -c example.cc
	ar rvs libmaster.a example.o
	ranlib libmaster.a

compile_worker:
	@echo "Building application for workers..."
	$(GCC) -DCOMPSS_WORKER $(CFLAGS) -c example.cc
	ar rvs libworker.a example.o
	ranlib libworker.a

deploy:
	@echo "Deploying lib files..."
	@rm -rf ../lib
	@mkdir ../lib
	@mv *.a ../lib

clean:
	@echo "Cleaning intermediate files..."
	rm -f *.o
