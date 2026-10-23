"""crop.py in.png out.webp x0 y0 x1 y1 [maxw] — crop (fractions 0..1 or px), resize to maxw, save webp q88."""
import sys
from PIL import Image
im = Image.open(sys.argv[1]).convert('RGB')
box = [float(v) for v in sys.argv[3:7]]
if all(v <= 1 for v in box):
    box = [box[0]*im.width, box[1]*im.height, box[2]*im.width, box[3]*im.height]
im = im.crop(tuple(int(v) for v in box))
maxw = int(sys.argv[7]) if len(sys.argv) > 7 else 2400
if im.width > maxw:
    im = im.resize((maxw, round(im.height*maxw/im.width)), Image.LANCZOS)
im.save(sys.argv[2], 'WEBP', quality=88, method=6)
print(sys.argv[2], im.size)
