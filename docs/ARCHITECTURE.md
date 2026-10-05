# SmartEYE arxitekturasi

SmartEYE bitta Go binardan iborat bo'lib, o'rnatish vaqtida tanlangan rol
(`server` yoki `client`) asosida turlicha ishlaydi. Rol va qurilma identifikatori
`%APPDATA%\SmartEYE\config.json` da saqlanadi.

## Komponentlar

```
cmd/smarteye            Kirish nuqtasi; rolni aniqlaydi va ishga tushiradi
internal/protocol       Simdagi xabarlar (JSON) + ramka (framed) ulanish
internal/config         Rol, qurilma ID, sozlamalar
internal/security       Self-signed TLS + tarmoq kodi (hash) juftlash
internal/discovery      UDP LAN beacon: server e'lon qiladi, client eshitadi
internal/screen         Ekran olish (Windows GDI; dev uchun o'rinbosar)
internal/control        Kiritish yuborish (Windows SendInput) + clipboard
internal/osops          Quvvat, dastur ochish, xabar, qulf ekrani, tizim ma'lumotlari
internal/agent          Client: topish, ulanish, oqim, buyruqlar, qayta ulanish
internal/server         Server: hub, agent tinglovchi, panel (embedded web)
```

## Aloqa oqimi

1. **Topish.** Server har ~2 soniyada UDP beacon yuboradi (o'z IP:porti va
   tarmoq kodining hashi bilan). Client mos hashli beacon'ni ko'rsa, ulanadi.
2. **Ulanish.** Client → server TLS TCP. Birinchi ramka `Hello` (qurilma
   ma'lumotlari + kod hashi). Server versiya va kodni tekshirib `Welcome`
   yoki `Reject` qaytaradi.
3. **Jonli ko'rinish.** Panel ochiq bo'lsa, server barcha agentlardan past
   tezlikdagi preview (thumbnail) so'raydi. Bitta kompyuter ochilsa, unga
   to'liq sifatli oqim (`stream_start`) yuboriladi.
4. **Boshqaruv.** Paneldagi amallar browser → server (WebSocket) → agent
   (TLS) orqali yetadi. Kiritish hodisalari normalangan (0..1) koordinatalarda
   yuboriladi, shuning uchun ekran o'lchamidan mustaqil.

## Transport

- **Agent ↔ Server:** 4 baytlik uzunlik + JSON (`internal/protocol/conn.go`).
  TLS 1.2+. Self-signed sertifikat; ishonch tarmoq kodi orqali o'rnatiladi.
- **Browser ↔ Server:** WebSocket (loopback), JSON `{type, payload}`.

## Xavfsizlik va ochiqlik tamoyillari

- Tarmoq kodisiz hech bir client serverga bo'ysunmaydi.
- Faqat lokal tarmoq; internetga chiqish yo'q.
- Client nazorat ostida ekanini bildiradi (ko'rinadigan belgi, standart yoniq).
- Yashirin kuzatuv, olib tashlashdan qochish yoki anti-tamper xususiyatlari
  ataylab qo'shilmagan.

## Kelgusi yaxshilanishlar

- Sertifikatni birinchi ulanishda pinlash (TOFU).
- Dirty-rect delta kodlash (hozir to'liq kadr JPEG).
- Session-0 xizmat + interaktiv sessiya yordamchisi (crash-restart watchdog).
- Doimiy tray belgisi va demo rejim overlay'i.
