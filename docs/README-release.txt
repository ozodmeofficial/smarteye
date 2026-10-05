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
- Server ishga tushganda boshqaruv paneli brauzerda ochiladi.
- Clientlar tarmoqda o'zi topiladi — IP kiritish shart emas.
- Kompyuterlarni xonalarga ajrating, tanlang va boshqaring:
  bloklash, xabar, dastur ochish, o'chirish, masofadan ko'rish/boshqarish.
- Kompyuterni ikki marta bosib, uning ekraniga ulaning va boshqaring.

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
