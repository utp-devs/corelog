package phone

// countryTable is the source of the country list. One country per line:
//
//	ISO|name (es)|dial code|NANP area codes (comma)|trunk prefixes (comma)|national length (min-max)|example
//
// Trunk prefixes are the domestic dialing prefixes that are never part of the international number
// (for example "0" in the United Kingdom). Countries where a leading zero belongs to the number (Italy)
// have no trunk prefix. The national length is only filled where it is fixed and well known; every
// number is additionally bound to 7-15 digits in total (E.164).
const countryTable = `AD|Andorra|376|||6-9|
AE|Emiratos Árabes Unidos|971||0|8-9|501234567
AF|Afganistán|93||0|9|
AG|Antigua y Barbuda|1|268||10|2684641234
AI|Anguila|1|264||10|2642351234
AL|Albania|355||0|8-9|
AM|Armenia|374||0|8|
AO|Angola|244|||9|
AQ|Antártida|672|||5-6|
AR|Argentina|54||0|10-11|1123456789
AS|Samoa Americana|1|684||10|6847331234
AT|Austria|43||0|9-13|
AU|Australia|61||0|9|412345678
AW|Aruba|297|||7|
AX|Islas Åland|358||0|5-10|
AZ|Azerbaiyán|994||0|9|
BA|Bosnia y Herzegovina|387||0|8|
BB|Barbados|1|246||10|2462501234
BD|Bangladés|880||0|10|
BE|Bélgica|32||0|8-9|
BF|Burkina Faso|226|||8|
BG|Bulgaria|359||0|8-9|
BH|Baréin|973|||8|
BI|Burundi|257|||8|
BJ|Benín|229|||8|
BL|San Bartolomé|590||0|9|
BM|Bermudas|1|441||10|4413701234
BN|Brunéi|673|||7|
BO|Bolivia|591|||8|71234567
BQ|Caribe Neerlandés|599|||7|
BR|Brasil|55||0|10-11|11912345678
BS|Bahamas|1|242||10|2423591234
BT|Bután|975|||8|
BV|Isla Bouvet|47|||8|
BW|Botsuana|267|||7-8|
BY|Bielorrusia|375||80|9|
BZ|Belice|501|||7|
CA|Canadá|1|||10|4165550123
CC|Islas Cocos|61||0|9|
CD|República Democrática del Congo|243||0|9|
CF|República Centroafricana|236|||8|
CG|Congo|242|||9|
CH|Suiza|41||0|9|
CI|Costa de Marfil|225|||8-10|
CK|Islas Cook|682|||5|
CL|Chile|56|||9|912345678
CM|Camerún|237|||9|
CN|China|86||0|11|13812345678
CO|Colombia|57|||10|3101234567
CR|Costa Rica|506|||8|83123456
CU|Cuba|53||0|8|51234567
CV|Cabo Verde|238|||7|
CW|Curazao|599|||7-8|
CX|Isla de Navidad|61||0|9|
CY|Chipre|357|||8|
CZ|Chequia|420|||9|
DE|Alemania|49||0|7-13|
DJ|Yibuti|253|||8|
DK|Dinamarca|45|||8|
DM|Dominica|1|767||10|7672251234
DO|República Dominicana|1|809,829,849||10|8092345678
DZ|Argelia|213||0|9|
EC|Ecuador|593||0|9|991234567
EE|Estonia|372|||7-8|
EG|Egipto|20||0|10|
EH|Sáhara Occidental|212||0|9|
ER|Eritrea|291|||7|
ES|España|34|||9|612345678
ET|Etiopía|251||0|9|
FI|Finlandia|358||0|5-11|
FJ|Fiyi|679|||7|
FK|Islas Malvinas|500|||5|
FM|Micronesia|691|||7|
FO|Islas Feroe|298|||6|
FR|Francia|33||0|9|612345678
GA|Gabón|241|||7-8|
GB|Reino Unido|44||0|10|7911123456
GD|Granada|1|473||10|4734031234
GE|Georgia|995||0|9|
GF|Guayana Francesa|594||0|9|
GG|Guernsey|44||0|10|
GH|Ghana|233||0|9|
GI|Gibraltar|350|||8|
GL|Groenlandia|299|||6|
GM|Gambia|220|||7|
GN|Guinea|224|||9|
GP|Guadalupe|590||0|9|
GQ|Guinea Ecuatorial|240|||9|
GR|Grecia|30|||10|
GS|Georgias del Sur e Islas Sandwich del Sur|500|||5|
GT|Guatemala|502|||8|51234567
GU|Guam|1|671||10|6713001234
GW|Guinea-Bisáu|245|||7|
GY|Guyana|592|||7|
HK|Hong Kong|852|||8|
HM|Islas Heard y McDonald|672|||5-6|
HN|Honduras|504|||8|91234567
HR|Croacia|385||0|8-9|
HT|Haití|509|||8|
HU|Hungría|36||06|8-9|
ID|Indonesia|62||0|9-11|
IE|Irlanda|353||0|7-9|
IL|Israel|972||0|8-9|
IM|Isla de Man|44||0|10|
IN|India|91||0|10|9123456789
IO|Territorio Británico del Océano Índico|246|||7|
IQ|Irak|964||0|9-10|
IR|Irán|98||0|10|
IS|Islandia|354|||7|
IT|Italia|39|||6-11|3123456789
JE|Jersey|44||0|10|
JM|Jamaica|1|876,658||10|8762101234
JO|Jordania|962||0|8-9|
JP|Japón|81||0|9-10|
KE|Kenia|254||0|9|
KG|Kirguistán|996||0|9|
KH|Camboya|855||0|8-9|
KI|Kiribati|686|||5|
KM|Comoras|269|||7|
KN|San Cristóbal y Nieves|1|869||10|8692361234
KP|Corea del Norte|850||0|8-10|
KR|Corea del Sur|82||0|9-10|
KW|Kuwait|965|||8|
KY|Islas Caimán|1|345||10|3453231234
KZ|Kazajistán|7||8|10|
LA|Laos|856||0|8-10|
LB|Líbano|961||0|7-8|
LC|Santa Lucía|1|758||10|7582841234
LI|Liechtenstein|423|||7|
LK|Sri Lanka|94||0|9|
LR|Liberia|231|||7-8|
LS|Lesoto|266|||8|
LT|Lituania|370||8|8|
LU|Luxemburgo|352|||6-9|
LV|Letonia|371|||8|
LY|Libia|218||0|9|
MA|Marruecos|212||0|9|
MC|Mónaco|377||0|8-9|
MD|Moldavia|373||0|8|
ME|Montenegro|382||0|8|
MF|San Martín|590||0|9|
MG|Madagascar|261||0|9|
MH|Islas Marshall|692|||7|
MK|Macedonia del Norte|389||0|8|
ML|Malí|223|||8|
MM|Myanmar|95||0|8-10|
MN|Mongolia|976||0|8|
MO|Macao|853|||8|
MP|Islas Marianas del Norte|1|670||10|6702345678
MQ|Martinica|596||0|9|
MR|Mauritania|222|||8|
MS|Montserrat|1|664||10|6644912345
MT|Malta|356|||8|
MU|Mauricio|230|||7-8|
MV|Maldivas|960|||7|
MW|Malaui|265||0|7-9|
MX|México|52|||10|5512345678
MY|Malasia|60||0|9-10|
MZ|Mozambique|258|||9|
NA|Namibia|264||0|9|
NC|Nueva Caledonia|687|||6|
NE|Níger|227|||8|
NF|Isla Norfolk|672|||6|
NG|Nigeria|234||0|10|
NI|Nicaragua|505|||8|81234567
NL|Países Bajos|31||0|9|
NO|Noruega|47|||8|
NP|Nepal|977||0|10|
NR|Nauru|674|||7|
NU|Niue|683|||4|
NZ|Nueva Zelanda|64||0|8-10|
OM|Omán|968|||8|
PA|Panamá|507|||7-8|61234567
PE|Perú|51||0|8-9|912345678
PF|Polinesia Francesa|689|||8|
PG|Papúa Nueva Guinea|675|||7-8|
PH|Filipinas|63||0|10|
PK|Pakistán|92||0|10|
PL|Polonia|48|||9|
PM|San Pedro y Miquelón|508||0|6|
PN|Islas Pitcairn|64|||8-9|
PR|Puerto Rico|1|787,939||10|7872345678
PS|Palestina|970||0|9|
PT|Portugal|351|||9|912345678
PW|Palaos|680|||7|
PY|Paraguay|595||0|9|
QA|Catar|974|||8|
RE|Reunión|262||0|9|
RO|Rumanía|40||0|9|
RS|Serbia|381||0|8-9|
RU|Rusia|7||8|10|
RW|Ruanda|250||0|9|
SA|Arabia Saudita|966||0|9|
SB|Islas Salomón|677|||5|
SC|Seychelles|248|||7|
SD|Sudán|249||0|9|
SE|Suecia|46||0|7-9|
SG|Singapur|65|||8|
SH|Santa Elena|290|||4-5|
SI|Eslovenia|386||0|8|
SJ|Svalbard y Jan Mayen|47|||8|
SK|Eslovaquia|421||0|9|
SL|Sierra Leona|232||0|8|
SM|San Marino|378|||6-10|
SN|Senegal|221|||9|
SO|Somalia|252|||7-9|
SR|Surinam|597|||6-7|
SS|Sudán del Sur|211||0|9|
ST|Santo Tomé y Príncipe|239|||7|
SV|El Salvador|503|||8|71234567
SX|Sint Maarten|1|721||10|7215204123
SY|Siria|963||0|8-9|
SZ|Esuatini|268|||8|
TC|Islas Turcas y Caicos|1|649||10|6492311234
TD|Chad|235|||8|
TF|Tierras Australes Francesas|262|||9|
TG|Togo|228|||8|
TH|Tailandia|66||0|8-9|
TJ|Tayikistán|992|||9|
TK|Tokelau|690|||4|
TL|Timor Oriental|670|||7-8|
TM|Turkmenistán|993||8|8|
TN|Túnez|216|||8|
TO|Tonga|676|||5|
TR|Turquía|90||0|10|
TT|Trinidad y Tobago|1|868||10|8682911234
TV|Tuvalu|688|||5|
TW|Taiwán|886||0|8-9|
TZ|Tanzania|255||0|9|
UA|Ucrania|380||0|9|
UG|Uganda|256||0|9|
UM|Islas menores de EE. UU.|1|||10|
US|Estados Unidos|1|||10|2025550123
UY|Uruguay|598||0|8|
UZ|Uzbekistán|998|||9|
VA|Ciudad del Vaticano|379|||6-10|
VC|San Vicente y las Granadinas|1|784||10|7842661234
VE|Venezuela|58||0|10|4121234567
VG|Islas Vírgenes Británicas|1|284||10|2843001234
VI|Islas Vírgenes de EE. UU.|1|340||10|3406421234
VN|Vietnam|84||0|9-10|
VU|Vanuatu|678|||5-7|
WF|Wallis y Futuna|681|||6|
WS|Samoa|685|||5-7|
YE|Yemen|967||0|7-9|
YT|Mayotte|262||0|9|
ZA|Sudáfrica|27||0|9|
ZM|Zambia|260||0|9|
ZW|Zimbabue|263||0|9|`
