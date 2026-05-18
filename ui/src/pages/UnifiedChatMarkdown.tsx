import ReactMarkdown from 'react-markdown';

export function AssistantMarkdown({ text }: { text: string }) {
  return (
    <div className="cht__markdown">
      <ReactMarkdown allowedElements={['p', 'strong', 'em', 'ul', 'ol', 'li', 'br', 'code', 'pre', 'blockquote']}>
        {text}
      </ReactMarkdown>
    </div>
  );
}
