# ChatGPT scheduled task prompt

Replace the scheduled task's prompt with the text below. It is the original prompt unchanged, plus a final paragraph block that asks for one copyable JSON block per event. The JSON keys are the contract with the app (`event/event.go`); if you rename a key here, rename it there too.

How to use the output: in the ChatGPT answer, press the copy button on the JSON block of an event you like, open the app, tap **+ Dodaj**, then **Zalijepi iz međuspremnika** (or paste manually) and **Spremi**. You can also paste several blocks, or the whole answer, at once — the app saves every event it finds and ignores the prose. Pasting an event that is already saved (same date and the same title or link) updates it instead of duplicating it.

---

```text
Pretraži web za najave događaja objavljene u prethodna 24 sata i sastavi praktičan dnevni popis mogućih događaja za vlasnika obrta ERIK SOFTWARE, koji pruža automatizacije, poslovni softver, web-stranice/webshopove i AI edukaciju malim i srednjim poduzetnicima u Hrvatskoj. Traži temeljito, uključujući službene kalendare, lokalne medije, komore/udruge obrtnika i poduzetnika, coworking/prostorne zajednice, fakultete, organizatore, LinkedIn, Facebook događaje i Eventbrite. Geografski fokus neka budu Rijeka, Opatija, Kastav, Crikvenica, otoci i ostatak Kvarnera; uključi Istru ili Gorski kotar kada je događaj osobito relevantan ili razumno dostupan iz Rijeke. Uključi konferencije, radionice, meetupe, doručke, networking, edukacije, sajmove i male neformalne skupove, i plaćene i besplatne. Prioritet su poduzetnici, obrtnici, vlasnici/menadžeri malih i srednjih poduzeća te pružatelji poslovnih usluga. Isključi usko strukovna događanja bez poslovne, tehnološke, marketinške ili digitalne komponente. Za svaki pronađeni događaj napiši: naziv, datum i vrijeme, mjesto, cijenu/uvjet prijave ako je poznat, tko će vjerojatno biti prisutan, zašto je dobar potencijal za umrežavanje te izravnu poveznicu na najavu ili prijavu. Jasno označi događaje koji su novi od prethodnog dana i odvoji najbolje prilike. Ako u prethodna 24 sata nema novih relevantnih najava, napiši to kratko i po potrebi navedi samo iznimno dobar već najavljen nadolazeći događaj koji bi bilo šteta propustiti.

Strojno čitljiv zapis: odmah ispod opisa svakog navedenog događaja (uključujući i eventualni rezervni ili već ranije najavljeni događaj) dodaj zaseban blok koda označen kao json. Svaki blok sadrži točno jedan JSON objekt za taj jedan događaj, bez komentara i bez ikakvog drugog teksta unutar bloka, kako bih ga mogao kopirati jednim klikom. Koristi točno ove ključeve, ovim redoslijedom:

{
  "title": "naziv događaja",
  "type": "jedno od: konferencija, radionica, meetup, doručak, networking, edukacija, predavanje, sajam, info dan, drugo",
  "start_date": "GGGG-MM-DD",
  "start_time": "HH:MM",
  "end_date": "GGGG-MM-DD",
  "end_time": "HH:MM",
  "city": "grad ili mjesto, npr. Rijeka",
  "venue": "naziv prostora i adresa",
  "organizer": "organizator",
  "price": "cijena i uvjet prijave, npr. Besplatno; prijava na adresa@primjer.hr do 10. 11.",
  "audience": "tko će vjerojatno biti prisutan",
  "why": "1–3 rečenice: zašto je dobar potencijal za umrežavanje i što je u programu relevantno",
  "url": "izravna poveznica na najavu ili prijavu"
}

Pravila za JSON:
- Datumi su u obliku GGGG-MM-DD, a vremena u 24-satnom obliku HH:MM po lokalnom vremenu (Zagreb). Ako najava ne navodi godinu, uzmi najbliži budući datum.
- end_date popuni samo za višednevne događaje; inače je null. end_time popuni samo ako je poznat.
- Sve što nije poznato iz najave napiši kao null; ne izmišljaj podatke.
- title i start_date su obavezni; događaj bez poznatog datuma ne stavljaj u JSON.
- Poveznica u "url" neka bude čista, bez utm i drugih parametara za praćenje.
- Tekstualne vrijednosti piši na hrvatskom, kratko i konkretno.
```

---

## Field reference

| Key | Required | Format | Shown as |
| --- | --- | --- | --- |
| `title` | yes | text | event title |
| `type` | no | short text | tag above the title |
| `start_date` | yes | `YYYY-MM-DD` | date column, grouping by month, upcoming/past |
| `start_time` | no | `HH:MM` | date column |
| `end_date` | no | `YYYY-MM-DD`, only for multi-day events | "do 20. 11." in the date column |
| `end_time` | no | `HH:MM` | "15:00–18:00" |
| `city` | no | text | tag above the title |
| `venue` | no | text | line under the title, "Gdje" |
| `organizer` | no | text | "Organizator" |
| `price` | no | text (price and how to register) | line under the title, "Cijena i prijava" |
| `audience` | no | text | "Tko dolazi" |
| `why` | no | text | "Zašto ići" |
| `url` | no | http(s) link | "Najava ↗" button |

The app is lenient with model output: `null` for any optional field, numbers instead of text, `9:00` or `15:00:00` for times, a combined `2026-11-19T15:00` in `start_date`, missing `https://`, `utm_*` parameters, the aliases `name`/`date`/`time`/`link`/`location`, and unknown extra keys are all accepted. Anything it cannot fix is reported in Croatian on the add page, with your pasted text kept.
