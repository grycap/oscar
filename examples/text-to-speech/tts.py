from google_speech import Speech
#import cv2
import os
import sys
import argparse

class SocialTools(object):

    def __init__(self, language):
        self.language=language
 

    def Text_ToFileMP3(self,text_mp3,filename):
        speech = Speech(text_mp3, self.language)
        speech.save(filename+".mp3")


def tts(input_arg,language,output):
	if os.path.exists(input_arg):
		text = open(input_arg, "r").read()
		transform(text,language,output)
	else:
		transform(str(input_arg),language,output)


def transform(text,language,output):
	so_good=SocialTools(language)
	so_good.Text_ToFileMP3(text,output)

if __name__ == "__main__":
	parser = argparse.ArgumentParser()
	parser.add_argument('-l','--language')
	parser.add_argument('FILE')
	parser.add_argument('-o', '--output', help='Save the result to a local file.')
	args = parser.parse_args()
	tts(args.FILE, args.language, args.output) 
