# OSCAR - On-premises Serverless Container-aware ARchitectures
# Copyright (C) GRyCAP - I3M - UPV
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import sys
import argparse
import os
import numpy as np
import video_frames as vf
import view_classification as vc
import doppler_segmentation as ds
import texture_analysis as tex
import textures_classification as tc

import warnings
warnings.filterwarnings('ignore')

# Set working directory
os.chdir(os.path.dirname(os.path.realpath(__file__)))

def classify_video(filename):
    # 1. If doppler, extract frames from video
    if not vf.if_doppler(filename):
        return 'Not doppler'
    else:
        frames = vf.load_video(filename)
        
        segmentations = []
        anatomics = []

        for fr in frames:
            # 2. Segment colors by frame
            segmentedImage, anatomicImage = ds.segmentation(fr)
            segmentations.append(segmentedImage)
            anatomics.append(anatomicImage)
        
        # 3. Classify view. If long axis, extract texture fetaures
        if not vc.if_long_axis(anatomics):
            return 'Not long axis'
        else:
            allTextures = []
            for s in segmentations:
                # 4. Texture analysis
                if np.max(s) == 0:
                    continue
                else:
                    textures = tex.textures(s)
                    allTextures.append(textures)
                
            # Calculate mean and median of the textures features plus max velocity of the sequence
            numberOfFeatures = len(allTextures[0])
            
            textureFeatures = np.zeros((1, numberOfFeatures*2 + 1))
            textureFeatures[0,:numberOfFeatures] = np.nanmean(np.array(allTextures), axis=0)
            textureFeatures[0,numberOfFeatures:-1] = np.nanmedian(np.array(allTextures), axis=0)
            textureFeatures[0,-1] = max(allTextures)[-2]
            
            # 5. Supervised classifier
            label = tc.classify(textureFeatures)[0] 
            
            if label == 1:
                return 'RHD'
            elif label == 0:
                return 'Normal'
            else:
                return 'Unspecific label'

if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('FILE')
    parser.add_argument('-o', '--output', help='Save the result to a local file.')
    args = parser.parse_args()

    result = classify_video(args.FILE)
    if args.output == None:
        print(result)
    else:
        with open(args.output, 'w') as f:
            f.write(os.path.basename(args.FILE) + ': ' + result + '\n')