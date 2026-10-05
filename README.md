# SmartEYE 👁

**Lokal tarmoq uchun kompyuterlarni nazorat qilish va masofadan boshqarish dasturi.**
Bitta dastur — ikki rol: *Server* (o'qituvchi / rahbar) va *Client* (o'quvchi / xodim).
Veyon'ning sinf nazorati va AnyDesk'ning masofadan boshqaruvi bitta chiroyli,
Claude uslubidagi panelda birlashtirilgan.

> **Ochiqlik:** SmartEYE yashirin josuslik vositasi emas. Nazorat ostidagi har bir
> kompyuter o'z holatini serverga bildiradi va (standart sozlamada) kimdir ulanganda
> foydalanuvchiga ko'rinadigan belgi chiqaradi. Dasturdan faqat qonuniy, oshkora
> nazorat (sinf, o'quv markazi, ish joyi) uchun foydalaning.

---

## ✨ Imkoniyatlar

**Nazorat (Veyon uslubida)**
- 🔍 Clientlar lokal tarmoqda **avtomatik topiladi** — IP qo'lda kiritilmaydi
- 🧩 Barcha ekranlar jonli plitkalar ko'rinishida; xonalarga ajratiladi
- 🔒 Bitta, tanlangan yoki barcha kompyuterlarni xabar bilan **bloklash**
- 💬 Xabar yuborish, ⚡ o'chirish/qayta yuklash/chiqish
- 🚀 Hammada bir vaqtda dastur yoki sayt ochish

**Masofadan boshqarish (AnyDesk uslubida)**
- 👁 Kuzatish va 🖱 to'liq boshqarish (sichqoncha + klaviatura)
- 🖥 Bir nechta monitor, ⌨️ Ctrl+Alt+Del, 📋 umumiy clipboard, 📸 skrinshot

**Qulaylik**
- 🖥 Server paneli alohida ilova oynasida ochiladi (brauzer emas, WebView2)
- 🔎 Avtomatik topilmasa — **IP bo'yicha qidirish** (yakka IP yoki oraliq)
- 🛡 O'rnatuvchi Windows Firewall ruxsatlarini avtomatik sozlaydi
- 🎨 Claude uslubidagi iliq dizayn, kunduzgi/tungi rejim
- 🌐 3 til: o'zbek, rus, ingliz
- 🔐 TLS shifrlash + tarmoq kodi orqali juftlash

---

## 🚀 O'rnatish

1. `SmartEYE-Setup.exe` ni ishga tushiring.
2. **Server** yoki **Client** rolini tanlang.
3. Server avtomatik **tarmoq kodi** yaratadi (masalan `482913`). Uni yozib oling.
4. Har bir client kompyuterda o'sha kodni kiriting — tamom.

Clientlar serverni o'zi topadi va kompyuter yonishi bilan fonda ishga tushadi.

---

## 🛠 Texnologiya

| Qism | Texnologiya |
|------|-------------|
| Yadro | Go (bitta `.exe`, cgo'siz) |
| Aloqa | TLS ustidan JSON ramkalar + WebSocket (panel) |
| Ekran | Windows GDI (BitBlt) |
| Kiritish | Windows SendInput |
| Topish | UDP LAN beacon |
| Panel | O'rnatilgan (embedded) vanilla SPA |
| O'rnatuvchi | Inno Setup |

Tafsilotlar: [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md).

---

## 🔨 Manbadan qurish

```bash
# Windows .exe (Linux yoki macOS'dan cross-compile):
build/build.sh 1.0.0        # -> dist/smarteye.exe

# Yoki to'g'ridan-to'g'ri:
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -ldflags "-H=windowsgui" -o smarteye.exe ./cmd/smarteye
```

Release (exe + o'rnatuvchi) `v*` tegi bilan GitHub Actions orqali avtomatik yig'iladi.

## ⚙️ CLI

```
smarteye                     # saqlangan rol bilan ishga tushirish
smarteye --setup --role server --name "3-xona" --code 482913
smarteye --setup --role client --code 482913
smarteye --version
```

## 📄 Litsenziya

MIT — `LICENSE` fayliga qarang.
