"use client";

import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

/** Renders model output as GitHub-flavored Markdown. Raw HTML is not rendered. */
export function MarkdownMessage({ content }: { content: string }) {
  return (
    <div className="chat-markdown text-sm leading-relaxed text-slate-800">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          a: ({ href, children }) => (
            <a
              href={href}
              target={href?.startsWith("http") ? "_blank" : undefined}
              rel={href?.startsWith("http") ? "noreferrer noopener" : undefined}
            >
              {children}
            </a>
          ),
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
}
