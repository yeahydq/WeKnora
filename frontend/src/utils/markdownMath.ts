export const preprocessMathDelimiters = (rawText: string): string => {
  if (!rawText || typeof rawText !== 'string') {
    return '';
  }

  const normalizeInlineMathBullets = (text: string): string => text
    .split('\n')
    .map((line) => {
      const bulletMatches = [...line.matchAll(/\s-\s+(\$[^$]+\$)/g)];
      if (!bulletMatches.length) return line;

      const firstMatch = bulletMatches[0];
      const prefix = typeof firstMatch.index === 'number' ? line.slice(0, firstMatch.index) : '';
      const hasIntroPrefix = /[：:]\s*$/.test(prefix);
      if (!hasIntroPrefix && bulletMatches.length < 2) {
        return line;
      }

      const pieces: string[] = [];
      const normalizedPrefix = prefix.trimEnd();
      if (normalizedPrefix) {
        pieces.push(normalizedPrefix, '');
      }

      for (const match of bulletMatches) {
        pieces.push(`- ${match[1]}`);
      }

      const lastMatch = bulletMatches[bulletMatches.length - 1];
      const lastIndex = typeof lastMatch.index === 'number' ? lastMatch.index + lastMatch[0].length : 0;
      const suffix = line.slice(lastIndex).trim();
      if (suffix) {
        pieces.push('', suffix);
      }

      return pieces.join('\n');
    })
    .join('\n');

  const normalized = normalizeInlineMathBullets(rawText)
    .replace(/\r\n?/g, '\n')
    // Promote headings that models sometimes append to the tail of a sentence.
    .replace(/([^\n])\s+(#{2,6}\s+)/g, '$1\n\n$2')
    // Ensure display-math fences become standalone markdown blocks so marked
    // does not keep the opening $$ inside the previous paragraph.
    .replace(/\\\[([\s\S]*?)\\\]/g, (_match, formula) => `\n\n$$\n${formula}\n$$\n\n`)
    .replace(/\\\(([\s\S]*?)\\\)/g, (_match, formula) => `$${formula}$`);

  // Some models emit display math using a lone `$` on its own line instead of
  // the standard `$$` fence. Promote only delimiter-only lines so we don't
  // rewrite normal inline math like `$x+y$` inside prose.
  const displaySafe = normalized.replace(/(^|\n)\s*\$\s*(?=\n|$)/g, (_match, prefix) => `${prefix}$$`);

  const lines = displaySafe.split('\n');
  const output: string[] = [];
  let inDisplayMath = false;

  const looksLikeStandaloneFormula = (trimmed: string): boolean => {
    if (!trimmed) return false;
    if (trimmed.startsWith('$$') || trimmed.startsWith('$')) return false;
    // Keep prose lines with inline math as normal markdown paragraphs.
    if (trimmed.includes('$')) return false;
    if (trimmed.startsWith('#')) return false;
    if (trimmed.startsWith('```')) return false;
    if (/^[-*+]\s/.test(trimmed)) return false;
    if (/^\d+\.\s/.test(trimmed)) return false;
    if (/^[（(]?\d+[）)]/.test(trimmed)) return false;

    // Strong signal: LaTeX control sequences in an otherwise standalone line.
    if (/\\(?:frac|text|times|cdot|mathrm|sqrt|sum|int|left|right|leq|geq|neq|approx|pi|alpha|beta|gamma|theta|lambda|mu|Delta|boxed|rho|sin|cos|tan|log|ln|overline|underline|vec|hat)/.test(trimmed)) {
      return true;
    }

    // Fallback: equation-like ASCII/CJK token line with operators/subscripts.
    if (/[=_^]/.test(trimmed) && /^[A-Za-z0-9_{}\\^=+\-*/().,:<>|\[\]\s\u4e00-\u9fff·%]+$/.test(trimmed)) {
      return true;
    }

    return false;
  };

  for (let i = 0; i < lines.length; i += 1) {
    const line = lines[i];
    const trimmed = line.trim();
    const prevTrimmed = output.length > 0 ? output[output.length - 1].trim() : '';
    const nextTrimmed = i + 1 < lines.length ? lines[i + 1].trim() : '';
    const isDelimiterOnly = trimmed === '$$' || trimmed === '$';
    const isSingleLineDisplay = /^\$\$[\s\S]+\$\$$/.test(trimmed);

    if (isDelimiterOnly) {
      if (!inDisplayMath && output.length > 0 && prevTrimmed !== '') {
        output.push('');
      }

      output.push(line);

      if (inDisplayMath && nextTrimmed !== '') {
        output.push('');
      }

      inDisplayMath = !inDisplayMath;
      continue;
    }

    if (isSingleLineDisplay) {
      if (output.length > 0 && prevTrimmed !== '') {
        output.push('');
      }
      output.push(line);
      if (nextTrimmed !== '') {
        output.push('');
      }
      continue;
    }

    if (!inDisplayMath && looksLikeStandaloneFormula(trimmed)) {
      if (output.length > 0 && prevTrimmed !== '') {
        output.push('');
      }
      output.push('$$');
      output.push(trimmed);
      output.push('$$');
      if (nextTrimmed !== '') {
        output.push('');
      }
      continue;
    }

    output.push(line);
  }

  return output.join('\n').replace(/\n{3,}/g, '\n\n');
};
