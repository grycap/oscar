
##########################################################################

# Example : perform live fire detection in image/video/webcam using
# NasNet-A-OnFire, ShuffleNetV2-OnFire CNN models.

# Copyright (c) 2020/21 - William Thompson / Neelanjan Bhowmik / Toby
# Breckon, Durham University, UK

# License :
# https://github.com/NeelBhowmik/efficient-compact-fire-detection-cnn/blob/main/LICENSE

##########################################################################

import cv2
import os
import sys
import math
from PIL import Image
import argparse
import time
import numpy as np
import math
import string
import random

import torch
import torchvision.transforms as transforms
from models import shufflenetv2
from models import nasnet_mobile_onfire
from pathlib import Path

# create random characters for filename
S = 10
randchars = ''.join(random.choices(string.ascii_uppercase + string.digits, k = S))

def write_output(prediction, output_file):
    if prediction == 1:
        res = "NO_FIRE"
    else:
        res = "FIRE"
    file = open(output_file, "x")
    file.write(res)

##########################################################################
# parse command line arguments
parser = argparse.ArgumentParser()
parser.add_argument("--image",
                    help="Path to image file or image directory")
parser.add_argument("--video",
                    help="Path to video file or video directory")
parser.add_argument(
    "--webcam",
    action="store_true",
    help="Take inputs from webcam")
parser.add_argument(
    "--camera_to_use",
    type=int,
    default=0,
    help="Specify camera to use for webcam option")
parser.add_argument("--trt",
                    action="store_true",
                    help="Model run on TensorRT")
parser.add_argument(
    "--model",
    default='shufflenetonfire',
    help="Select the model {shufflenetonfire, nasnetonfire}")
parser.add_argument("--weight", help="Model weight file path")
parser.add_argument(
    "--cpu",
    action="store_true",
    help="If selected will run on CPU")
parser.add_argument(
    "--output",
    help="A directory to save output visualizations."
    "If not given , will show output in an OpenCV window.")
parser.add_argument(
    "-fs",
    "--fullscreen",
    action='store_true',
    help="run in full screen mode")
args = parser.parse_args()
print(f'\n{args}')
##########################################################################

# inference_ff.py methods

# model prediction on image
def run_model_img(args, frame, model):
    output = model(frame)
    pred = torch.round(torch.sigmoid(output))
    return pred

def data_transform(model):
    # transforms needed for shufflenetonfire
    if model == 'shufflenetonfire':
        np_transforms = transforms.Compose([
            transforms.ToTensor(),
            transforms.Normalize((0.485, 0.456, 0.406), (0.229, 0.224, 0.225))
        ])
    # transforms needed for nasnetonfire
    if model == 'nasnetonfire':
        np_transforms = transforms.Compose([
            transforms.ToTensor(),
            transforms.Normalize((0.5, 0.5, 0.5), (0.5, 0.5, 0.5))
        ])

    return np_transforms

# read/process image and apply tranformation
def read_img(frame, np_transforms):
    small_frame = cv2.resize(frame, (224, 224), cv2.INTER_AREA)
    small_frame = cv2.cvtColor(small_frame, cv2.COLOR_BGR2RGB)
    small_frame = Image.fromarray(small_frame)
    small_frame = np_transforms(small_frame).float()
    small_frame = small_frame.unsqueeze(0)
    small_frame = small_frame.to(device)

    return small_frame

##########################################################################

# inference_superpixel.py methods

def proc_sp(small_frame, np_transforms):
    # small_frame = cv2.resize(frame, (224, 224), cv2.INTER_AREA)
    small_frame = Image.fromarray(small_frame)
    small_frame = np_transforms(small_frame).float()
    small_frame = small_frame.unsqueeze(0)
    small_frame = small_frame.to(device)

    return small_frame

# drawing prediction on image
def draw_pred(args, frame, contours, prediction):
    # height, width, _ = frame.shape
    if prediction == 1:
        cv2.drawContours(frame, contours, -1, (0, 0, 255), 1)
    else:
        cv2.drawContours(frame, contours, -1, (0, 255, 0), 1)
    return frame

def process_sp(args, small_frame, np_transforms, model):
    # apply SLIC superpixel
    slic = cv2.ximgproc.createSuperpixelSLIC(small_frame, region_size=22)
    slic.iterate(10)
    segments = slic.getLabels()

    for (i, segVal) in enumerate(np.unique(segments)):
        mask = np.zeros(small_frame.shape[:2], dtype='uint8')
        mask[segments == segVal] = 255

        if (int(cv2.__version__.split(".")[0]) >= 4):
            contours, hierarchy = cv2.findContours(
                mask, cv2.RETR_TREE, cv2.CHAIN_APPROX_SIMPLE)
        else:
            im2, contours, hierarchy = cv2.findContours(
                mask, cv2.RETR_TREE, cv2.CHAIN_APPROX_SIMPLE)

        # contour_list.append(contours)
        superpixel = cv2.bitwise_and(small_frame, small_frame, mask=mask)
        superpixel = cv2.cvtColor(superpixel, cv2.COLOR_BGR2RGB)

        # PIL centre crop and data transformation
        # superpixel = pil_crop(superpixel)
        superpixel = proc_sp(superpixel, np_transforms)

        # model prediction
        prediction = run_model_img(args, superpixel, model)

        # draw prediction on superpixel
        draw_pred(args, small_frame, contours, prediction)

##########################################################################

# uses cuda if available
if args.cpu:
    device = torch.device('cpu')
else:
    device = torch.device('cuda:0' if torch.cuda.is_available() else 'cpu')

if args.cpu and args.trt:
    print(f'\n>>>>TensorRT runs only on gpu. Exit.')
    exit()

# model load
if args.model == "shufflenetonfire":
    model = shufflenetv2.shufflenet_v2_x0_5(
        pretrained=False, layers=[
            4, 8, 4], output_channels=[
            24, 48, 96, 192, 64], num_classes=1)
    if args.weight:
        w_path = args.weight
    else:
        w_path = './weights/shufflenet_sp.pt'
    model.load_state_dict(torch.load(w_path, map_location=device))
elif args.model == "nasnetonfire":
    model = nasnet_mobile_onfire.nasnetamobile(num_classes=1, pretrained=False)
    if args.weight:
        w_path = args.weight
    else:
        w_path = './weights/nasnet_sp.pt'
    model.load_state_dict(torch.load(w_path, map_location=device))
else:
    print('Invalid Model.')
    exit()

# apply data transform
np_transforms = data_transform(args.model)


model.eval()
model.to(device)

# TensorRT conversion
if args.trt:
    from torch2trt import TRTModule
    from torch2trt import torch2trt
    data = torch.randn((1, 3, 224, 224)).float().to(device)
    model_trt = torch2trt(model, [data], int8_mode=True)
    model_trt.to(device)

# load and process input image directory or image file
if args.image:

    # list image from a directory or file
    if os.path.isdir(args.image):
        lst_img = [os.path.join(args.image, file)
                   for file in os.listdir(args.image)]
    if os.path.isfile(args.image):
        lst_img = [args.image]

    if args.output:
        os.makedirs(args.output, exist_ok=True)

    # start processing image
    for im in lst_img:
        start_t = time.time()
        frame = cv2.imread(im)
        height, width, _ = frame.shape

        infs_small_frame = cv2.resize(frame, (224, 224), cv2.INTER_AREA)
        inff_small_frame = read_img(frame, np_transforms)

        # model prediction
        if args.trt:
            prediction = run_model_img(args, inff_small_frame, model_trt)
            process_sp(args, infs_small_frame, np_transforms, model_trt)
        else:
            prediction = run_model_img(args, inff_small_frame, model)
            process_sp(args, infs_small_frame, np_transforms, model)

        stop_t = time.time()
        
        # save prdiction visualisation in output path
        infs_small_frame = cv2.resize(infs_small_frame, (width, height), cv2.INTER_AREA)
        
        file = Path(im)
        file_txt = file.with_suffix('.txt')
        f_name = os.path.basename(file)
        f_name_txt = os.path.basename(file_txt)

        output_file = args.output+"/"+"output-"+f_name_txt
        output_image = "output-"+f_name
        print(output_file)
        print(output_image)
        cv2.imwrite(f'{args.output}/{output_image}', infs_small_frame)

        # save prediction result in file
        write_output(prediction, output_file)

