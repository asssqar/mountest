export type Subject = {
  id: string;
  name: string;
  variantsCount: number;
};

export type Variant = {
  id: string;
  subjectId: string;
  title: string;
  durationMinutes: number;
  questionsCount: number;
};

export type AnswerOption = {
  id: string;
  text: string;
  position: number;
  isCorrect: boolean;
};

export type Question = {
  id: string;
  variantId: string;
  position: number;
  text: string;
  options: AnswerOption[];
};

export type Guest = {
  id: string;
  firstName: string;
  lastName: string;
};

export type Attempt = {
  id: string;
  variantId: string;
  variantTitle: string;
  subjectName: string;
  durationMinutes: number;
  startedAt: string;
  finishedAt: string | null;
  questions: Question[];
  answers: Record<string, string[]>;
  guest?: Guest;
};

export type ResultErrorEntry = {
  questionId: string;
  questionText: string;
  options: AnswerOption[];
  selectedOptionIds: string[];
  correctOptionIds: string[];
};

export type AttemptResult = {
  attemptId: string;
  score: number;
  total: number;
  startedAt: string;
  finishedAt: string;
  errors: ResultErrorEntry[];
  guest?: Guest;
};
