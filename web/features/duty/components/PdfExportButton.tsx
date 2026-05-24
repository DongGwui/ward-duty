"use client";
import { Button } from "@/components/ui/Button";

interface Props {
  /** 캡처할 영역의 ID (SchedulerGrid 컨테이너의 id 속성). */
  targetId: string;
  filename: string;
  format?: "pdf" | "png";
  disabled?: boolean;
}

export function PdfExportButton({ targetId, filename, format = "pdf", disabled }: Props) {
  const onClick = async () => {
    const el = document.getElementById(targetId);
    if (!el) return;
    // dynamic import — 번들 크기 절약
    const [{ default: html2canvas }, { default: jsPDF }] = await Promise.all([
      import("html2canvas"),
      import("jspdf"),
    ]);
    const canvas = await html2canvas(el, { scale: 2, backgroundColor: "#ffffff" });
    if (format === "png") {
      const link = document.createElement("a");
      link.href = canvas.toDataURL("image/png");
      link.download = `${filename}.png`;
      link.click();
      return;
    }
    const pdf = new jsPDF({ orientation: "landscape", unit: "px", format: [canvas.width, canvas.height] });
    pdf.addImage(canvas.toDataURL("image/png"), "PNG", 0, 0, canvas.width, canvas.height);
    pdf.save(`${filename}.pdf`);
  };
  return (
    <Button variant="secondary" size="sm" onClick={onClick} disabled={disabled}>
      {format === "pdf" ? "PDF 내보내기" : "이미지 내보내기"}
    </Button>
  );
}
