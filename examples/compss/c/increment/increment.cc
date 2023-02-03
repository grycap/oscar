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

#include "increment.h"

using namespace std;

#define FILE_NAME1 "file1.txt"
#define FILE_NAME2 "file2.txt"
#define FILE_NAME3 "file3.txt"

void usage() {
    cerr << "[ERROR] Bad number of parameters" << endl;
    cout << "    Usage: increment <numIterations> <counterValue1> <counterValue2> <counterValue3>" << endl;
}

void initializeCounters(string counter1, string counter2, string counter3, file fileName1, file fileName2, file fileName3) {
    // Write file1
    ofstream fos1 (fileName1);
    if (fos1.is_open()) {
        fos1 << counter1 << endl;
        fos1.close();
    } else {
        cerr << "[ERROR] Unable to open file" << endl;
        return;
    }

    // Write file2
    ofstream fos2 (fileName2);
    if (fos2.is_open()) {
        fos2 << counter2 << endl;
        fos2.close();
    } else {
        cerr << "[ERROR] Unable to open file" << endl;
        return;
    }

    // Write file3
    ofstream fos3 (fileName3);
    if (fos3.is_open()) {
        fos3 << counter3 << endl;
        fos3.close();
    } else {
        cerr << "[ERROR] Unable to open file" << endl;
        return;
    }
}

void printCounterValues(file fileName1, file fileName2, file fileName3) {
    // Read new value from file 1
    string value1;
    ifstream fis1;
    compss_ifstream(fileName1, fis1);
    if (fis1.is_open()) {
        if (getline(fis1, value1)) {
            cout << "- Counter1 value is " << value1 << endl;
            fis1.close();
        } else {
            cerr << "[ERROR] Unable to read counter1 value" << endl;
            fis1.close();
            return;
        }
    } else {
        cerr << "[ERROR] Unable to open file" << endl;
        return;
    }

    // Read new value from file 2
    string value2;
    ifstream fis2;
    compss_ifstream(fileName2, fis2); 
    if (fis2.is_open()) {
        if (getline(fis2, value2)) {
            cout << "- Counter2 value is " << value2 << endl;
            fis2.close();
        } else {
            cerr << "[ERROR] Unable to read counter2 value" << endl;
            fis2.close();
            return;
        }
    } else {
        cerr << "[ERROR] Unable to open file" << endl;
        return;
    }
 
    // Read new value from file 3
    string value3;
    ifstream fis3;
    compss_ifstream(fileName3, fis3); 
    if (fis3.is_open()) {
        if (getline(fis3, value3)) {
            cout << "- Counter3 value is " << value3 << endl;
            fis3.close();
        } else {
            cerr << "[ERROR] Unable to read counter3 value" << endl;
            fis3.close();
            return;
        }
    } else {
        cerr << "[ERROR] Unable to open file" << endl;
        return;
    }
}

int main(int argc, char *argv[]) {
    // Check and get parameters
    if (argc != 5) {
        usage();
        return -1;
    }
    int N = atoi( argv[1] );
    string counter1 = argv[2];
    string counter2 = argv[3];
    string counter3 = argv[4];

    // Init COMPSs
    compss_on();

    // Initialize counter files
    file fileName1 = strdup(FILE_NAME1);
    file fileName2 = strdup(FILE_NAME2);
    file fileName3 = strdup(FILE_NAME3);
    initializeCounters(counter1, counter2, counter3, fileName1, fileName2, fileName3);

    // Print initial counters state
    cout << "Initial counter values: " << endl;
    printCounterValues(fileName1, fileName2, fileName3);

    // Execute increment tasks
    for (int i = 0; i < N; ++i) {
        increment(fileName1);
        increment(fileName2);
        increment(fileName3);
    }

    // Print final state
    cout << "Final counter values: " << endl;
    printCounterValues(fileName1, fileName2, fileName3);

    // Stop COMPSs
    compss_off();

    return 0;
}

