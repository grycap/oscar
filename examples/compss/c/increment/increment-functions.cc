/*
 *  Copyright 2002-2015 Barcelona Supercomputing Center (www.bsc.es)
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */
#include<iostream>
#include<fstream>
#include<string>
#include"increment.h"


void increment(file fileName) {
    std::cout << "INIT TASK" << std::endl;
    std::cout << "Param: " << fileName << std::endl;

    int value;
    // Read value
    std::ifstream fis(fileName);
    if (fis.is_open()) {        
        if (fis >> value) {
            fis.close();
        } else {
            std::cerr << "[ERROR] Unable to read final value" << std::endl;
            fis.close();
        }
        fis.close();
    } else {
        std::cerr << "[ERROR] Unable to open file" << std::endl;
    }
    
    // Increment
    std::cout << "INIT VALUE: " << value << std::endl;
    std::cout << "FINAL VALUE: " << ++value << std::endl;
    
    // Write new value
    std::ofstream fos (fileName);
    if (fos.is_open()) {
        fos << value << std::endl;
        fos.close();
    } else {
        std::cerr << "[ERROR] Unable to open file" << std::endl;
    }
    std::cout << "END TASK" << std::endl;
}
