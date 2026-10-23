"""trim.py in.png out.png — crop away the stage background (sampled at the corner)."""
import sys
from PIL import Image, ImageChops
im = Image.open(sys.argv[1]).convert('RGB')
bg = Image.new('RGB', im.size, im.getpixel((2, 2)))
diff = ImageChops.difference(im, bg).convert('L').point(lambda v: 255 if v > 12 else 0)
box = diff.getbbox()
if box: im = im.crop(box)
im.save(sys.argv[2]); print(sys.argv[2], im.size)
