import { GoogleGenerativeAI } from "@google/generative-ai";

export async function POST(request: Request) {
  try {
    const formData = await request.formData();
    const audioBlob = formData.get('audio') as Blob;

    if (!audioBlob) {
      return Response.json({ error: 'No audio data provided' }, { status: 400 });
    }

    if (!process.env.GOOGLE_API_KEY) {
      console.error('Google API key not found');
      return Response.json({ error: 'API key not configured' }, { status: 500 });
    }

    // Convert Blob to base64
    const buffer = await audioBlob.arrayBuffer();
    const base64Audio = Buffer.from(buffer).toString('base64');

    const genAI = new GoogleGenerativeAI(process.env.GOOGLE_API_KEY);
    const model = genAI.getGenerativeModel({ model: "gemini-2.0-flash-exp" });

    const result = await model.generateContent([
      {
        inlineData: {
          mimeType: audioBlob.type || 'audio/webm',
          data: base64Audio
        }
      },
      { text: "Generate a transcript of the speech. Do not include any other text or formatting." },
    ]);

    return Response.json({ transcript: result.response.text() });

  } catch (error) {
    console.error('Transcription error:', error);
    return Response.json({ error: 'Transcription failed' }, { status: 500 });
  }
} 