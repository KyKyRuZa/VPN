import styles from "./Instructions.module.css";

const items = [
  {
    title: "Android",
    desc: "Скачайте v2rayNG или FoXray, добавьте ключ через «Добавить профиль».",
  },
  { title: "iOS", desc: "Shadowrocket / FoXray: отсканируйте QR или импортируйте ссылку." },
  { title: "Windows", desc: "v2rayN / NekoRay: добавьте VLESS Reality конфиг." },
  { title: "macOS / Linux", desc: "v2rayA / Clash Verge: импортируйте подписку." },
];

export default function Instructions() {
  return (
    <div className="section" style={{ padding: 0 }}>
      <div className={styles.grid}>
        {items.map((it) => (
          <div key={it.title} className={styles.item}>
            <h3>{it.title}</h3>
            <p>{it.desc}</p>
          </div>
        ))}
      </div>
    </div>
  );
}
