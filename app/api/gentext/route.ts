import { GoogleGenerativeAI } from "@google/generative-ai";

export async function GET() {
  try {
    if (!process.env.GOOGLE_API_KEY) {
      console.error('Google API key not found');
      return Response.json({ error: 'API key not configured' }, { status: 500 });
    }

    const genAI = new GoogleGenerativeAI(process.env.GOOGLE_API_KEY);
    const model = genAI.getGenerativeModel({ model: "gemini-2.0-flash-exp" });
    const prompt = "Generate a short, magical story about books or reading (2-3 sentences). Make it whimsical and engaging. DO NOT OUTPUT ANYTHING OTHER THAN THE STORY";

    const result = await model.generateContent([
      { text: prompt },
    ]);

    const text = result.response.text();

    return Response.json({ story: text });

  } catch (error) {
    console.error('Story generation error:', error);
    return Response.json({ error: 'Story generation failed' }, { status: 500 });
  }
}
