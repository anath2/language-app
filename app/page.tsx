'use client';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faMicrophone, faStop, faRotate } from '@fortawesome/free-solid-svg-icons';
import { useState, useRef, useEffect } from 'react';
import { TextComparison } from './components/TextComparison';

export default function Home() {
  const [isRecording, setIsRecording] = useState(false);
  const [transcription, setTranscription] = useState<string>('');
  const [audioUrl, setAudioUrl] = useState<string | null>(null);
  const [story, setStory] = useState<string>('');
  const [isLoadingStory, setIsLoadingStory] = useState(true);
  const [isLoadingTranscription, setIsLoadingTranscription] = useState(false);
  const mediaRecorderRef = useRef<MediaRecorder | null>(null);
  const audioChunksRef = useRef<Blob[]>([]);

  const fetchNewStory = async () => {
    setIsLoadingStory(true);
    try {
      // Add artificial delay to test loading state
      await new Promise(resolve => setTimeout(resolve, 1000));
      
      const response = await fetch('/api/gentext');
      if (!response.ok) throw new Error('Failed to fetch story');
      const data = await response.json();
      setStory(data.story);
    } catch (error) {
      console.error('Error fetching story:', error);
      setStory('Error loading story. Please try again.');
    } finally {
      setIsLoadingStory(false);
    }
  };

  useEffect(() => {
    fetchNewStory();
  }, []);

  useEffect(() => {
    return () => {
      if (audioUrl) {
        URL.revokeObjectURL(audioUrl);
      }
    };
  }, [audioUrl]);

  const startRecording = async () => {
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      const mediaRecorder = new MediaRecorder(stream);
      
      mediaRecorder.ondataavailable = (event) => {
        if (event.data.size > 0) {
          audioChunksRef.current.push(event.data);
        }
      };

      mediaRecorder.onstop = async () => {
        const audioBlob = new Blob(audioChunksRef.current, { type: 'audio/webm' });
        const url = URL.createObjectURL(audioBlob);
        setAudioUrl(url);
        
        // Send complete recording for transcription
        await sendRecordingForTranscription(audioBlob);
      };

      mediaRecorder.start(1000);
      mediaRecorderRef.current = mediaRecorder;
      setIsRecording(true);
      setTranscription('');
      audioChunksRef.current = []; // Clear previous chunks
      
      if (audioUrl) {
        URL.revokeObjectURL(audioUrl);
        setAudioUrl(null);
      }
    } catch (error) {
      console.error('Error accessing microphone:', error);
    }
  };

  const stopRecording = () => {
    if (mediaRecorderRef.current) {
      mediaRecorderRef.current.stop();
      mediaRecorderRef.current.stream.getTracks().forEach(track => track.stop());
      setIsRecording(false);
    }
  };

  const sendRecordingForTranscription = async (audioBlob: Blob) => {
    try {
      const formData = new FormData();
      formData.append('audio', audioBlob);
      setIsLoadingTranscription(true);
      const response = await fetch('/api/transcribe', {
        method: 'POST',
        body: formData,
      });

      if (!response.ok) {
        throw new Error('Transcription request failed');
      }
      const data = await response.json();
      if (data.transcript) {
        setTranscription(data.transcript);
      }
    } catch (error) {
      console.error('Error sending recording:', error);
    } finally {
      setIsLoadingTranscription(false);
    }
  };

  const handleToggleRecording = () => {
    if (isRecording) {
      stopRecording();
    } else {
      startRecording();
    }
  };

  const handleNewStory = () => {
    setTranscription('');  // Clear previous transcription
    setAudioUrl(null);     // Clear previous audio
    fetchNewStory();       // Get new story
  };

  return (
    <div className="grid grid-rows-[20px_1fr_20px] items-center justify-items-center min-h-screen p-8 pb-20 gap-16 sm:p-20 font-[family-name:var(--font-geist-sans)]">
      <main className="flex flex-col gap-8 row-start-2 items-center">
        <div className="max-w-2xl mx-auto px-6 relative">
          {isLoadingStory || isLoadingTranscription ? (
            <div className="w-full animate-pulse bg-gray-200 dark:bg-gray-700 rounded-lg">
              &nbsp;&nbsp;
            </div>
          ) : transcription ? (
            <TextComparison 
              originalText={story}
              transcribedText={transcription}
            />
          ) : (
            <>
              <p className="italic text-2xl sm:text-3xl leading-relaxed text-gray-700 dark:text-gray-300 font-[family-name:var(--font-playfair)] text-center">
                "{story}"
              </p>
            </>
          )}
        </div>

        <div className="flex flex-col items-center gap-4">
          {transcription ? (
            <button
              onClick={handleNewStory}
              className="px-10 py-4 rounded-full text-white bg-gray-800 hover:bg-gray-700 dark:bg-gray-200 dark:text-gray-800 dark:hover:bg-gray-300 transition-colors duration-200 text-lg flex items-center gap-3"
            >
              Try New Story
              <FontAwesomeIcon icon={faRotate} className="w-4 h-4" />
            </button>
          ) : (
            <button 
              onClick={handleToggleRecording}
              className={`px-10 py-4 rounded-full text-white transition-colors duration-200 text-lg flex items-center gap-3
                ${isRecording 
                  ? 'bg-red-600 hover:bg-red-700 dark:bg-red-500 dark:hover:bg-red-600' 
                  : 'bg-gray-800 hover:bg-gray-700 dark:bg-gray-200 dark:text-gray-800 dark:hover:bg-gray-300'
                }`}
            >
              {isRecording ? 'Stop' : 'Speak'}
              <FontAwesomeIcon 
                icon={isRecording ? faStop : faMicrophone} 
                className="w-4 h-4" 
              />
            </button>
          )}

          {audioUrl && (
            <audio 
              controls 
              src={audioUrl}
              className="w-64 h-12 mt-4"
            />
          )}
        </div>
      </main>
    </div>
  );
}
