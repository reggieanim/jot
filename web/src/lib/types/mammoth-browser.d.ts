declare module 'mammoth/mammoth.browser' {
	export type ExtractRawTextResult = {
		value: string;
		messages: Array<{ type: string; message: string }>;
	};

	export type ConvertToHtmlResult = {
		value: string;
		messages: Array<{ type: string; message: string }>;
	};

	export function extractRawText(input: { arrayBuffer: ArrayBuffer }): Promise<ExtractRawTextResult>;
	export function convertToHtml(input: { arrayBuffer: ArrayBuffer }): Promise<ConvertToHtmlResult>;
}
