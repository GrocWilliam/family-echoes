// Client-side image preparation before upload. Doing this in the browser
// rather than the Go API gets us, for free, EXIF orientation handling and
// decoding of every format the browser itself understands — notably HEIC on
// Safari, which is exactly where iPhone photos come from.

// Grid tiles are ~220 CSS px wide at most; 480 covers 2x screens.
const THUMB_MAX_SIDE = 480;
// HEIC photos are re-encoded as JPEG. iOS caps canvas area at ~16.7M pixels,
// so very large (e.g. 48MP) photos must be scaled down to fit.
const CONVERTED_MAX_SIDE = 4096;
// Must match maxPhotoSize in backend/handlers/handler_photo.go.
export const MAX_UPLOAD_BYTES = 20 * 1024 * 1024;

export class UploadError extends Error {}

export interface PreparedUpload {
	photo: Blob;
	fileName: string;
	thumbnail: Blob | null;
}

interface DecodedImage {
	source: CanvasImageSource;
	width: number;
	height: number;
	release: () => void;
}

function isHeic(file: File): boolean {
	return /^image\/hei[cf]/i.test(file.type) || /\.hei[cf]$/i.test(file.name);
}

async function decode(file: Blob): Promise<DecodedImage | null> {
	try {
		const bitmap = await createImageBitmap(file);
		return {
			source: bitmap,
			width: bitmap.width,
			height: bitmap.height,
			release: () => bitmap.close()
		};
	} catch {
		// Some browsers decode formats in <img> that createImageBitmap rejects.
	}

	const url = URL.createObjectURL(file);
	try {
		const img = new Image();
		img.src = url;
		await img.decode();
		return {
			source: img,
			width: img.naturalWidth,
			height: img.naturalHeight,
			release: () => URL.revokeObjectURL(url)
		};
	} catch {
		URL.revokeObjectURL(url);
		return null;
	}
}

function encodeJpeg(image: DecodedImage, maxSide: number, quality: number): Promise<Blob | null> {
	const scale = Math.min(1, maxSide / Math.max(image.width, image.height));
	const canvas = document.createElement('canvas');
	canvas.width = Math.max(1, Math.round(image.width * scale));
	canvas.height = Math.max(1, Math.round(image.height * scale));

	const context = canvas.getContext('2d');
	if (!context) return Promise.resolve(null);
	context.drawImage(image.source, 0, 0, canvas.width, canvas.height);

	return new Promise((resolve) => canvas.toBlob(resolve, 'image/jpeg', quality));
}

export async function prepareUpload(file: File): Promise<PreparedUpload> {
	// Reject before decoding: a huge non-HEIC file is uploaded as-is, so
	// decoding it would only waste the phone's memory.
	if (!isHeic(file) && file.size > MAX_UPLOAD_BYTES) {
		throw new UploadError('trop volumineuse (max 20 Mo)');
	}

	const image = await decode(file);
	try {
		let photo: Blob = file;
		let fileName = file.name;

		if (isHeic(file)) {
			const converted = image ? await encodeJpeg(image, CONVERTED_MAX_SIDE, 0.9) : null;
			if (!converted) {
				throw new UploadError('format HEIC non pris en charge par ce navigateur');
			}
			photo = converted;
			fileName = file.name.replace(/\.hei[cf]$/i, '') + '.jpg';
		}

		if (photo.size > MAX_UPLOAD_BYTES) {
			throw new UploadError('trop volumineuse (max 20 Mo)');
		}

		// Best-effort: without a thumbnail the grid just shows the full photo.
		const thumbnail = image ? await encodeJpeg(image, THUMB_MAX_SIDE, 0.8) : null;

		return { photo, fileName, thumbnail };
	} finally {
		image?.release();
	}
}
