SmartEYE — lokal tarmoq nazorati va masofadan boshqaruv
========================================================

O'RNATISH
---------
1. SmartEYE-Setup.exe ni ishga tushiring (administrator huquqi bilan).
2. Rolni tanlang:
   - SERVER  : o'qituvchi / rahbar kompyuteri (hammani shu yerdan ko'rasiz).
   - CLIENT  : o'quvchi / xodim kompyuteri (fon rejimida ishlaydi).
3. SERVER avtomatik TARMOQ KODINI yaratadi (masalan 482913). Uni yozib oling.
4. Har bir CLIENT kompyuterda o'rnatishda shu kodni kiriting.

ISHLATISH
---------
- Server ishga tushganda boshqaruv paneli ALOHIDA OYNADA ochiladi (brauzer emas).
- Clientlar tarmoqda o'zi topiladi — IP kiritish shart emas.
- Agar avtomatik topilmasa: yuqoridagi WiFi belgisidagi tugma orqali
  "IP bo'yicha qidirish" — kompyuter IP manzillarini kiriting (masalan
  192.168.1.20 yoki 192.168.1.10-40 oralig'i).
- Kompyuterlarni xonalarga ajrating, tanlang va boshqaring:
  bloklash, xabar, dastur ochish, o'chirish, masofadan ko'rish/boshqarish.
- Kompyuterni ikki marta bosib, uning ekraniga ulaning va boshqaring.

TOPILMAYAPTIMI?
---------------
- O'rnatuvchi Windows Firewall ruxsatlarini avtomatik qo'shadi. Agar portativ
  (zip) versiyadan foydalansangiz, birinchi ishga tushishda Windows "ruxsat
  berish" so'rasa — "Allow access" ni bosing (ham Private, ham Public).
- Server va barcha clientlar bitta WiFi/tarmoqda ekaniga ishonch hosil qiling.
- Ba'zi WiFi routerlarda "client isolation" yoqilgan bo'ladi — bunda avtomatik
  topish ishlamaydi; "IP bo'yicha qidirish" dan foydalaning.

PORTATIV REJIM (o'rnatuvchisiz)
------------------------------
smarteye.exe --setup --role server --code 482913 --name "3-xona"
smarteye.exe --setup --role client --code 482913
Keyin: smarteye.exe

ESLATMA
-------
SmartEYE faqat lokal tarmoqda ishlaydi va oshkora nazorat uchun mo'ljallangan.
Nazorat ostidagi kompyuterda belgi ko'rinadi. Faqat o'zingizga tegishli yoki
sizga ishonib topshirilgan kompyuterlarda, egalarini ogohlantirgan holda ishlating.

Loglar: %APPDATA%\SmartEYE\smarteye.log
