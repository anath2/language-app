interface TextComparisonProps {
  originalText: string;
  transcribedText: string;
}

export function TextComparison({ originalText, transcribedText }: TextComparisonProps) {
  const compareWords = () => {
    const originalWords = originalText.toLowerCase().match(/[\w']+/g) || [];
    const transcribedWords = new Set(transcribedText.toLowerCase().match(/[\w']+/g) || []);
    
    let currentIndex = 0;
    let result = originalText;

    for (const word of originalWords) {
      const wordIndex = result.toLowerCase().indexOf(word, currentIndex);
      if (wordIndex === -1) continue;

      const isCorrect = transcribedWords.has(word);
      const originalWord = result.slice(wordIndex, wordIndex + word.length);
      const coloredWord = `<span class="${isCorrect ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'}">${originalWord}</span>`;
      
      result = result.slice(0, wordIndex) + coloredWord + result.slice(wordIndex + word.length);
      currentIndex = wordIndex + coloredWord.length;
    }

    return result;
  };

  return (
    <div className="max-w-2xl mx-auto px-6">
      <p 
        className="italic text-2xl sm:text-3xl leading-relaxed text-gray-700 dark:text-gray-300 font-[family-name:var(--font-playfair)] text-center"
        dangerouslySetInnerHTML={{ __html: compareWords() }}
      />
    </div>
  );
} 